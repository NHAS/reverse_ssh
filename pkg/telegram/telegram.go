package telegram

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

type BotInfos struct {
	Id                      int    `json:"id"`
	IsBot                   bool   `json:"is_bot"`
	FirstName               string `json:"first_name"`
	Username                string `json:"username"`
	CanJoinGroups           bool   `json:"can_join_groups"`
	CanReadAllGroupMessages bool   `json:"can_read_all_group_messages"`
	SupportsInlineQueries   bool   `json:"supports_inline_queries"`
}

type GetMeResponse struct {
	Ok     bool     `json:"ok"`
	Result BotInfos `json:"result"`
}

type GetChatResponse struct {
	Ok     bool `json:"ok"`
	Result struct {
		Id        int    `json:"id"`
		FirstName string `json:"first_name"`
		Username  string `json:"username"`
		Type      string `json:"type"`
		Photo     struct {
			SmallFileId       string `json:"small"`
			SmallFileUniqueId string `json:"small_file_unique_id"`
			BigFileId         string `json:"big"`
			BigFileUniqueId   string `json:"big_file_unique_id"`
		} `json:"photo"`
	} `json:"result"`
}

type SendMessageResponse struct {
	Ok     bool `json:"ok"`
	Result struct {
		MessageId int `json:"message_id"`
		From      struct {
			Id        int    `json:"id"`
			FirstName string `json:"first_name"`
			Username  string `json:"username"`
		} `json:"from"`
		Chat struct {
			Id        int    `json:"id"`
			FirstName string `json:"first_name"`
			Username  string `json:"username"`
			Type      string `json:"type"`
		} `json:"chat"`
		Date int    `json:"date"`
		Text string `json:"text"`
	} `json:"result"`
	ErrorCode   int    `json:"error_code"`
	Description string `json:"description"`
}

var telegram_enabled = false
var telegramToken string
var telegramChatId int
var botInfos BotInfos

func Start(token string, chatId int) {
	if token == "" && chatId == 0 {
		return
	}

	if token == "" || chatId == 0 {
		log.Print("Telegram is disabled: telegram_token AND telegram_chat_id must be setted")
		return
	}

	telegramInitError, _ := Init(token, chatId)

	if telegramInitError != nil {
		log.Printf("Telegram init failed: %s", telegramInitError)
	} else {
		log.Printf("Telegram bot (%s) is ready to send messages", botInfos.FirstName)
		SendMessage("Reverse SSH server is ready to receive connections")
	}

}

func Init(token string, chatId int) (error, bool) {
	getMeError, _ := GetMe(token)
	if getMeError != nil {
		telegram_enabled = false
		return getMeError, false
	}
	telegramToken = token

	getChatError, _ := GetChat(token, chatId)
	if getChatError != nil {
		telegram_enabled = false
		return getChatError, false
	}
	telegramChatId = chatId
	telegram_enabled = true

	return nil, true
}

func GetMe(token string) (error, *GetMeResponse) {

	resp, err := http.Get("https://api.telegram.org/bot" + token + "/getMe")
	if err != nil {
		return errors.New(fmt.Sprintf("Error getting telegram bot infos: %s", err)), nil
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.New(fmt.Sprintf("Error reading telegram bot infos: %s", err)), nil
	}

	res := GetMeResponse{}
	if err := json.Unmarshal(body, &res); err != nil {
		return errors.New(fmt.Sprintf("Error on parsing Telegram body response: %s", err)), nil
	}
	botInfos = res.Result

	return nil, &res
}

func GetChat(token string, chatId int) (error, *GetChatResponse) {

	resp, err := http.Get("https://api.telegram.org/bot" + token + "/getChat?chat_id=" + fmt.Sprint(chatId))
	if err != nil {
		return errors.New(fmt.Sprintf("Error getting telegram bot infos: %s", err)), nil
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.New(fmt.Sprintf("Error reading telegram bot infos: %s", err)), nil
	}

	res := GetChatResponse{}
	if err := json.Unmarshal(body, &res); err != nil {
		return errors.New(fmt.Sprintf("Error on parsing Telegram body response: %s", err)), nil
	}

	return nil, &res
}

func SendMessage(text string) (error, *SendMessageResponse) {
	if !telegram_enabled {
		return errors.New("Telegram is disabled"), nil
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage?chat_id=%d&text=%s&parse_mode=markdown", telegramToken, telegramChatId, text)

	method := "GET"
	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)

	if err != nil {
		return errors.New(fmt.Sprintf("Error creating request: %s", err)), nil
	}
	res, err := client.Do(req)
	if err != nil {
		return errors.New(fmt.Sprintf("Error sending request: %s", err)), nil
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.New(fmt.Sprintf("Error reading response: %s", err)), nil
	}

	resp := SendMessageResponse{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return errors.New(fmt.Sprintf("Error on parsing Telegram body response: %s", err)), nil
	}

	if resp.Ok == false {
		return errors.New(fmt.Sprintf("Error on sending message: %s", resp.Description)), nil
	}

	return nil, &resp
}
