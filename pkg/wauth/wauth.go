//go:build windows

package wauth

import (
	"encoding/base64"
	"fmt"
	"log"
	"strings"
	"syscall"
	"unsafe"
)

var (
	modsecur32 = syscall.NewLazyDLL("secur32.dll")

	procAcquireCredentialsHandleW  = modsecur32.NewProc("AcquireCredentialsHandleW")
	procFreeCredentialsHandle      = modsecur32.NewProc("FreeCredentialsHandle")
	procInitializeSecurityContextW = modsecur32.NewProc("InitializeSecurityContextW")

	errors = map[int64]string{
		0x80090300: "SEC_E_INSUFFICIENT_MEMORY",
		0x80090304: "SEC_E_INTERNAL_ERROR",
		0x8009030E: "SEC_E_NO_CREDENTIALS",
		0x80090306: "SEC_E_NOT_OWNER",
		0x80090305: "SEC_E_SECPKG_NOT_FOUND",
		0x8009030D: "SEC_E_UNKNOWN_CREDENTIALS",
		0x80090301: "SEC_E_INVALID_HANDLE",
		0x80090308: "SEC_E_INVALID_TOKEN",
		0x8009030C: "SEC_E_LOGON_DENIED",
		0x80090311: "SEC_E_NO_AUTHENTICATING_AUTHORITY",
		0x80090303: "SEC_E_TARGET_UNKNOWN",
		0x80090302: "SEC_E_UNSUPPORTED_FUNCTION",
		0x80090322: "SEC_E_WRONG_PRINCIPAL",
		0x00090314: "SEC_I_COMPLETE_AND_CONTINUE",
		0x00090312: "SEC_I_CONTINUE_NEEDED",
		0x00090313: "SEC_I_COMPLETE_NEEDED"}
)

func orPanic(err error) {
	if err != nil {
		panic(err)
	}
}

func AcquireCredentialsHandle(principal *uint16, pckge *uint16, credentialuse uint32, logonid *uint64, authdata *byte, getkeyfn *byte, getkeyargument *byte, credential *CredHandle, expiry *TimeStamp) (status SECURITY_STATUS) {
	r0, _, _ := syscall.Syscall9(procAcquireCredentialsHandleW.Addr(), 9, uintptr(unsafe.Pointer(principal)), uintptr(unsafe.Pointer(pckge)), uintptr(credentialuse), uintptr(unsafe.Pointer(logonid)), uintptr(unsafe.Pointer(authdata)), uintptr(unsafe.Pointer(getkeyfn)), uintptr(unsafe.Pointer(getkeyargument)), uintptr(unsafe.Pointer(credential)), uintptr(unsafe.Pointer(expiry)))
	status = SECURITY_STATUS(r0)
	return
}

func FreeCredentialsHandle(credential *CredHandle) (status SECURITY_STATUS) {
	r0, _, _ := syscall.Syscall(procFreeCredentialsHandle.Addr(), 1, uintptr(unsafe.Pointer(credential)), 0, 0)
	status = SECURITY_STATUS(r0)
	return
}

func InitializeSecurityContext(credential *CredHandle, context *CtxtHandle, targetname *uint16, contextreq uint32, reserved1 uint32, targetdatarep uint32, input *SecBufferDesc, reserved2 uint32, newcontext *CtxtHandle, output *SecBufferDesc, contextattr *uint32, expiry *TimeStamp) (status SECURITY_STATUS) {
	r0, _, _ := syscall.Syscall12(procInitializeSecurityContextW.Addr(), 12, uintptr(unsafe.Pointer(credential)), uintptr(unsafe.Pointer(context)), uintptr(unsafe.Pointer(targetname)), uintptr(contextreq), uintptr(reserved1), uintptr(targetdatarep), uintptr(unsafe.Pointer(input)), uintptr(reserved2), uintptr(unsafe.Pointer(newcontext)), uintptr(unsafe.Pointer(output)), uintptr(unsafe.Pointer(contextattr)), uintptr(unsafe.Pointer(expiry)))
	status = SECURITY_STATUS(r0)
	return
}

const (
	SECPKG_CRED_AUTOLOGON_RESTRICTED = 0x00000010
	SECPKG_CRED_BOTH                 = 0x00000003
	SECPKG_CRED_INBOUND              = 0x00000001
	SECPKG_CRED_OUTBOUND             = 0x00000002
	SECPKG_CRED_PROCESS_POLICY_ONLY  = 0x00000020
	SEC_E_OK                         = 0x00000000
	ISC_REQ_ALLOCATE_MEMORY          = 0x00000100
	ISC_REQ_CONNECTION               = 0x00000800
	ISC_REQ_INTEGRITY                = 0x00010000
	SECURITY_NATIVE_DREP             = 0x00000010
	SECURITY_NETWORK_DREP            = 0x00000000
	ISC_REQ_CONFIDENTIALITY          = 0x00000010
	ISC_REQ_REPLAY_DETECT            = 0x00000004

	SECBUFFER_TOKEN = 2
)

type SECURITY_STATUS int32

func (s SECURITY_STATUS) IsError() bool {
	return s < 0
}

func (s SECURITY_STATUS) IsInformation() bool {
	return s > 0
}

type Error int32

func (e Error) Error() string {
	return fmt.Sprintf("error #%x", uint32(e))
}

type CredHandle struct {
	Lower *uint32
	Upper *uint32
}

type CtxtHandle struct {
	Lower *uint32
	Upper *uint32
}

type SecBuffer struct {
	Count  uint32
	Type   uint32
	Buffer *byte
}

type SecBufferDesc struct {
	Version uint32
	Count   uint32
	Buffers *SecBuffer
}

// TimeStamp is an alias to Filetime (available only on Windows)
type TimeStamp syscall.Filetime

type Credentials struct {
	Handle CredHandle
}

func AcquireCredentials(username string) (*Credentials, SECURITY_STATUS, error) {
	var h CredHandle
	s := AcquireCredentialsHandle(nil, syscall.StringToUTF16Ptr("Negotiate"),
		SECPKG_CRED_OUTBOUND, nil, nil, nil, nil, &h, nil)
	if s.IsError() {
		return nil, s, Error(s)
	}
	return &Credentials{Handle: h}, s, nil
}

func (c *Credentials) Close() error {
	s := FreeCredentialsHandle(&c.Handle)
	if s.IsError() {
		return Error(s)
	}
	return nil
}

type Context struct {
	Handle     CtxtHandle
	BufferDesc SecBufferDesc
	Buffer     SecBuffer
	Data       [4096]byte // If the token will be larger than this the request will fail
	Attrs      uint32
}

func (c *Credentials) NewContext(target string) (*Context, SECURITY_STATUS, error) {
	var x Context
	x.Buffer.Buffer = &x.Data[0]
	x.Buffer.Count = uint32(len(x.Data))
	x.Buffer.Type = SECBUFFER_TOKEN
	x.BufferDesc.Count = 1
	x.BufferDesc.Buffers = &x.Buffer
	s := InitializeSecurityContext(&c.Handle, nil, syscall.StringToUTF16Ptr(target),
		ISC_REQ_CONFIDENTIALITY|ISC_REQ_REPLAY_DETECT|ISC_REQ_CONNECTION,
		0, SECURITY_NETWORK_DREP, nil,
		0, &x.Handle, &x.BufferDesc, &x.Attrs, nil)
	if s.IsError() {
		return nil, s, Error(s)
	}
	return &x, s, nil
}

func GetAuthorizationHeader(proxyURL string) string {

	// Acquire credentials
	cred, status, err := AcquireCredentials("")
	if err != nil {
		log.Printf("AcquireCredentials failed: %v %s", err, errors[int64(status)])
	}
	defer cred.Close()
	log.Printf("AcquireCredentials success: status=0x%x", status)

	// Initialize Context
	tgt := "http/" + strings.ToUpper(strings.Replace(strings.Split(proxyURL, ":")[1], "//", "", -1))
	log.Printf("Requesting for context against SPN %s", tgt)
	ctxt, status, err := cred.NewContext(tgt)

	if err != nil {
		log.Printf("NewContext failed: %v", err)
	}
	log.Printf("NewContext success: status=0x%x errorcode=%s", status, errors[int64(status)])

	// Generate the Authorization header
	headerstr := "Negotiate " + base64.StdEncoding.EncodeToString(ctxt.Data[0:ctxt.Buffer.Count])
	log.Printf("Generated header %s", headerstr)

	return headerstr
}
