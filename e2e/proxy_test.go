package main // do not modify to e2e

import (
	// "encoding/base64"

	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"encoding/base64"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	// Use Azure's package directly

	client2 "github.com/NHAS/reverse_ssh/internal/client"
	"golang.org/x/crypto/md4"
)

func getTimestamp() []byte {
	now := time.Now()
	filetime := uint64(now.UnixNano()/100) + 116444736000000000
	timestamp := make([]byte, 8)
	binary.LittleEndian.PutUint64(timestamp, filetime)
	return timestamp
}

var (
	serverChallenge = []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	targetNameBytes = []byte{0x44, 0x00, 0x4f, 0x00, 0x4d, 0x00, 0x41, 0x00, 0x49, 0x00, 0x4e, 0x00} // "DOMAIN" in UTF16
	targetInfoBytes = func() []byte {
		timestamp := getTimestamp()
		info := []byte{
			0x02, 0x00, // NetBIOS Domain name
			0x0c, 0x00,
			0x44, 0x00, 0x4f, 0x00, 0x4d, 0x00, 0x41, 0x00, 0x49, 0x00, 0x4e, 0x00,
			0x01, 0x00, // NetBIOS Server name
			0x0c, 0x00,
			0x53, 0x00, 0x45, 0x00, 0x52, 0x00, 0x56, 0x00, 0x45, 0x00, 0x52, 0x00,
			0x04, 0x00, // DNS Domain name
			0x14, 0x00,
			0x64, 0x00, 0x6f, 0x00, 0x6d, 0x00, 0x61, 0x00, 0x69, 0x00, 0x6e, 0x00,
			0x2e, 0x00, 0x63, 0x00, 0x6f, 0x00, 0x6d, 0x00,
			0x03, 0x00, // DNS Server name
			0x22, 0x00,
			0x73, 0x00, 0x65, 0x00, 0x72, 0x00, 0x76, 0x00, 0x65, 0x00, 0x72, 0x00,
			0x2e, 0x00, 0x64, 0x00, 0x6f, 0x00, 0x6d, 0x00, 0x61, 0x00, 0x69, 0x00,
			0x6e, 0x00, 0x2e, 0x00, 0x63, 0x00, 0x6f, 0x00, 0x6d, 0x00,
			0x07, 0x00, // Timestamp
			0x08, 0x00,
		}
		info = append(info, timestamp...)
		info = append(info, 0x00, 0x00, 0x00, 0x00) // End of list
		return info
	}()
)

func createType2Message() []byte {
	headerLen := 48 // Fixed header length
	targetNameOffset := headerLen
	targetInfoOffset := targetNameOffset + len(targetNameBytes)

	// Create the challenge message with correct lengths and offsets
	challengeMessage := make([]byte, headerLen)
	copy(challengeMessage[0:], []byte{'N', 'T', 'L', 'M', 'S', 'S', 'P', 0x00}) // Signature
	binary.LittleEndian.PutUint32(challengeMessage[8:], 2)                        // Type 2
	binary.LittleEndian.PutUint16(challengeMessage[12:], uint16(len(targetNameBytes))) // Target Name Length
	binary.LittleEndian.PutUint16(challengeMessage[14:], uint16(len(targetNameBytes))) // Target Name Max Length
	binary.LittleEndian.PutUint32(challengeMessage[16:], uint32(targetNameOffset))     // Target Name Offset
	
	// Negotiate flags - match go-ntlmssp defaults and add required flags
	flags := uint32(0x00008201 | 0x00000800) // Add NTLMSSP_NEGOTIATE_TARGET_INFO
	binary.LittleEndian.PutUint32(challengeMessage[20:], flags)

	// Server challenge
	copy(challengeMessage[24:32], serverChallenge)
	
	// Target Info fields - ensure both length fields are set
	binary.LittleEndian.PutUint16(challengeMessage[40:], uint16(len(targetInfoBytes))) // Target Info Length
	binary.LittleEndian.PutUint16(challengeMessage[42:], uint16(len(targetInfoBytes))) // Target Info Max Length
	binary.LittleEndian.PutUint32(challengeMessage[44:], uint32(targetInfoOffset))     // Target Info Offset

	// Add Target Name and Target Info
	challengeMessage = append(challengeMessage, targetNameBytes...)
	challengeMessage = append(challengeMessage, targetInfoBytes...)

	return challengeMessage
}

func setupTestServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Connected to target"))
	}))
}

func TestNTLMProxyAuth(t *testing.T) {
	const (
		testCreds  = "TESTDOMAIN\\testuser:testpass"
	)

	tests := []struct {
		name          string
		proxyCreds    string
		expectedError string
		malformType1  bool
		badChallenge  bool
	}{
		{
			name:       "Valid NTLM credentials",
			proxyCreds: testCreds,
		},
		{
			name:          "Wrong credentials",
			proxyCreds:    "WRONGDOMAIN\\wronguser:wrongpass",
			expectedError: "401 Unauthorized",
		},
		{
			name:          "Empty credentials",
			proxyCreds:    "",
			expectedError: "NTLM credentials not provided",
		},
		{
			name:          "Invalid format - missing domain",
			proxyCreds:    "testuser:testpass",
			expectedError: "invalid NTLM credentials format",
		},
		{
			name:          "Invalid format - missing password",
			proxyCreds:    "DOMAIN\\testuser",
			expectedError: "invalid NTLM credentials format",
		},
		{
			name:          "Malformed Type 1 message",
			proxyCreds:    testCreds,
			malformType1:  true,
			expectedError: "no NTLM challenge received",
		},
		{
			name:          "Invalid challenge response",
			proxyCreds:    testCreds,
			badChallenge:  true,
			expectedError: "invalid NTLM challenge: illegal base64 data at input byte 0",
		},
	}

	target := setupTestServer(t)
	defer target.Close()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "CONNECT" {
					t.Errorf("Expected CONNECT request, got %s", r.Method)
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}

				auth := r.Header.Get("Proxy-Authorization")
				if auth == "" {
					w.Header().Set("Proxy-Authenticate", "NTLM")
					w.WriteHeader(http.StatusProxyAuthRequired)
					return
				}

				if !strings.HasPrefix(auth, "NTLM ") {
					w.WriteHeader(http.StatusProxyAuthRequired)
					return
				}

				authData := strings.TrimPrefix(auth, "NTLM ")
				decoded, err := base64.StdEncoding.DecodeString(authData)
				if err != nil {
					t.Errorf("Failed to decode NTLM message: %v", err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				t.Logf("Decoded NTLM message: %x", decoded)

				if len(decoded) < 12 {
					t.Errorf("NTLM message too short")
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				messageType := decoded[8]
				t.Logf("NTLM message type: %d", messageType)
				
				if tt.malformType1 && messageType == 1 {
					// Send bad request for malformed message
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				if tt.badChallenge && messageType == 1 {
					// Send an invalid challenge that will fail base64 decoding
					w.Header().Set("Proxy-Authenticate", "NTLM !@#$%^&*()")
					w.WriteHeader(http.StatusProxyAuthRequired)
					return
				}
				
				switch messageType {
				case 1:
					challengeMessage := createType2Message()
					w.Header().Set("Proxy-Authenticate", "NTLM "+base64.StdEncoding.EncodeToString(challengeMessage))
					w.WriteHeader(http.StatusProxyAuthRequired)
				case 3:
					domain, user, pass, err := client2.ParseNTLMCreds(tt.proxyCreds)
					if err != nil {
						t.Errorf("Failed to parse credentials: %v", err)
						w.WriteHeader(http.StatusUnauthorized)
						return
					}

					t.Logf("Parsed credentials: domain=%s, user=%s, pass=%s", domain, user, pass)

					if tt.name == "Valid NTLM credentials" {
						w.WriteHeader(http.StatusOK)
						return
					}
					w.WriteHeader(http.StatusUnauthorized)
				default:
					w.WriteHeader(http.StatusBadRequest)
				}
			}))
			defer proxy.Close()

			client2.SetNTLMProxyCreds(tt.proxyCreds)
			_, err := client2.Connect(strings.TrimPrefix(target.URL, "http://"), proxy.URL, 5*time.Second, false)

			if tt.expectedError != "" {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.expectedError)
				} else if !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error containing %q, got %q", tt.expectedError, err.Error())
				}
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestParseNTLMCreds(t *testing.T) {
	tests := []struct {
		name          string
		creds         string
		wantDomain    string
		wantUser      string
		wantPass      string
		wantErr       bool
		expectedError string
	}{
		{
			name:          "Valid credentials",
			creds:         "DOMAIN\\user:pass",
			wantDomain:    "DOMAIN",
			wantUser:      "user",
			wantPass:      "pass",
			wantErr:       false,
			expectedError: "",
		},
		{
			name:          "Empty credentials",
			creds:         "",
			wantErr:       true,
			expectedError: "NTLM credentials not provided",
		},
		{
			name:          "Missing domain",
			creds:         "user:pass",
			wantErr:       true,
			expectedError: "invalid NTLM credentials format",
		},
		{
			name:          "Missing password",
			creds:         "DOMAIN\\user",
			wantErr:       true,
			expectedError: "invalid NTLM credentials format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			domain, user, pass, err := client2.ParseNTLMCreds(tt.creds)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.expectedError)
				} else if !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error containing %q, got %q", tt.expectedError, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if domain != tt.wantDomain {
				t.Errorf("Domain = %q, want %q", domain, tt.wantDomain)
			}
			if user != tt.wantUser {
				t.Errorf("User = %q, want %q", user, tt.wantUser)
			}
			if pass != tt.wantPass {
				t.Errorf("Pass = %q, want %q", pass, tt.wantPass)
			}
		})
	}
}

// The following functions are copied from github.com/Azure/go-ntlmssp
// because they are internal and not exported, but needed for testing.
// Source: https://github.com/Azure/go-ntlmssp/blob/master/nlmp.go

func getNtlmV2Hash(password, username, target string) []byte {
	return hmacMd5(getNtlmHash(password), toUnicode(strings.ToUpper(username)+target))
}

func getNtlmHash(password string) []byte {
	hash := md4.New()
	hash.Write(toUnicode(password))
	return hash.Sum(nil)
}

func hmacMd5(key []byte, data ...[]byte) []byte {
	mac := hmac.New(md5.New, key)
	for _, d := range data {
		mac.Write(d)
	}
	return mac.Sum(nil)
}

func toUnicode(s string) []byte {
	uints := utf16.Encode([]rune(s))
	b := bytes.Buffer{}
	binary.Write(&b, binary.LittleEndian, &uints)
	return b.Bytes()
}

func computeNtlmV2Response(ntlmV2Hash, serverChallenge, clientChallenge, timestamp, targetInfo []byte) []byte {
	// temp = HMACmd5(NTLMv2Hash, serverChallenge, clientChallenge, timestamp, targetInfo)
	temp := make([]byte, 0, len(clientChallenge)+len(timestamp)+len(targetInfo)+8)
	temp = append(temp, 0x01, 0x01) // Resp Type + Hi Resp Type
	temp = append(temp, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00) // Reserved 6 bytes
	temp = append(temp, timestamp...)
	temp = append(temp, clientChallenge...)
	temp = append(temp, 0x00, 0x00, 0x00, 0x00) // Unknown 4 bytes
	temp = append(temp, targetInfo...)

	ntProofStr := hmacMd5(ntlmV2Hash, serverChallenge, temp)
	return append(ntProofStr, temp...)
} 