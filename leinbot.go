package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

type Keys struct {
	Token    string   `json: "token"`
	Admins   []string `json: "admins"`
	Guilds   []string `json: "guilds"`
	Channels []string `json: "channels"`
}

var (
	commandPrefix string
	botID         string
	keys          Keys
)

func errCheck(msg string, err error) {
	if err != nil {
		fmt.Printf("%s: %+v\n", msg, err)
		panic(err)
	}
}

func main() {
	fmt.Println("Hello, I'm LeiNBot!!")
	bs, err := ioutil.ReadFile("prop.json")
	errCheck("Ошибка открытия файла параметров", err)
	err = json.Unmarshal(bs, &keys)
	errCheck("Ошибка получения параметров из JSON", err)
	discord, err := discordgo.New("Bot " + keys.Token)
	errCheck("error creating discord session", err)
	user, err := discord.User("@me")
	errCheck("error retrieving account", err)

	botID = user.ID
	discord.AddHandler(recieveMessage)

	discord.AddHandler(func(discord *discordgo.Session, ready *discordgo.Ready) {
		err = discord.UpdateStatus(0, "The LeiN guild lottery bot!\n")
		if err != nil {
			fmt.Println("Error attempting to set my status\n")
		}
		servers := discord.State.Guilds
		fmt.Printf("LeinBot has started on %d servers\n", len(servers))
	})

	err = discord.Open()
	errCheck("Error opening connection to Discord\n", err)
	defer discord.Close()

	commandPrefix = "!lottery"

	// SUPER hacky way of making our main function sit and wait forever while not using any CPU
	<-make(chan struct{})

}

func checkRights(guildID, channelID, author string) bool {
	sort.Strings(keys.Guilds)
	i := sort.SearchStrings(keys.Guilds, guildID)
	if i >= len(keys.Guilds) || keys.Guilds[i] != guildID {
		log.Printf("Гильдии %s нет в списке разрешённных", guildID)
		return false
	}
	sort.Strings(keys.Channels)
	i = sort.SearchStrings(keys.Channels, channelID)
	if i >= len(keys.Channels) || keys.Channels[i] != channelID {
		log.Printf("Канала %s нет в списке разрешённных", channelID)
		return false
	}
	sort.Strings(keys.Admins)
	i = sort.SearchStrings(keys.Admins, author)
	if i >= len(keys.Admins) || keys.Admins[i] != author {
		log.Printf("Пользователя %s нет в списке разрешённных", author)
		return false
	}
	return true
}

func recieveMessage(discord *discordgo.Session, message *discordgo.MessageCreate) {
	user := message.Author
	if user.ID == botID || user.Bot {
		//Do nothing because the bot is talking
		return
	}

	if !checkRights(message.GuildID, message.ChannelID, message.Author.String()) {
		return
	}

	if strings.HasPrefix(message.Content, commandPrefix) {
		go StartCommand(discord, message)
	}

	fmt.Printf("Message: \"%+v\" from: %s\n", message.Content, message.Author.Username)
}

func StartCommand(dg *discordgo.Session, m *discordgo.MessageCreate) {
	text := strings.Fields(m.Content)
	var botMessage string
	if len(text) > 1 {
		switch text[1] {
		// Перечень списков розыгрышей
		case "list":
			botMessage = listLottery()
		// Создать новый список розыгрыша
		case "create":
		// Удалить существующий список розыгрышей
		case "delete":
		// Показать список участников розыгрыша
		case "show":
		// Добавить участника в список розыгрыша
		case "add":
		// Удалить участника из списка розыгрыша
		case "remove":
		// Текущие данные по розыгрышу
		case "status":
		// Установка параметров розыгрыша
		case "params":
		// Старт розыгрыша
		case "start":
		// Справка по командам бота
		case "help":
			botMessage = "Справка по командам бота:\n\n" +
				"**list**\nПеречень списков розыгрышей\n" +
				"**create**\nСоздать новый список розыгрыша\n" +
				"**delete**\nУдалить существующий список розыгрышей\n" +
				"**show**\nПоказать список участников розыгрыша\n" +
				"**add**\nДобавить участника в список розыгрыша\n" +
				"**remove**\nУдалить участника из списка розыгрыша\n" +
				"**status**\nУстановка параметров розыгрыша\n" +
				"**params**\nУстановка параметров розыгрыша\n" +
				"**start**\nСтарт розыгрыша\n" +
				"**help**\nСправка по командам бота\n"
		}
		if botMessage != "" {
			dg.ChannelMessageSend(m.ChannelID, botMessage)
		}
	} else {
		dg.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Hello, %s!", m.Author.Username))
	}
}

func listLottery() string {
	files, err := filepath.Glob("lottery-*.csv")
	if err != nil {
		log.Print("Произошла ошибка: %s", err)
		return ""
	}
	text := "Найдено розыгрышей: " + strconv.Itoa(len(files)) + "\n"
	text += strings.Join(files, "\n")
	return text
}
