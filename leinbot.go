package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
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

type LotteryType int

const (
	TournamentLottery LotteryType = 0 // Тип розыгрыша - по местам
	DrawLottery       LotteryType = 1 // Тип розыгрыша - по количеству билетов у участника
)

type Tournament struct {
	Point   int    // Место
	Members int    // Количество победителей на этом месте
	Prize   string // Приз
}

type Lottery struct {
	Type        LotteryType
	Tournaments []Tournament
}

var (
	commandPrefix string
	filePrefix    string
	botID         string
	keys          Keys
	lotteries     []Lottery
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
	filePrefix = "lottery"

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
		// Перечень списков розыгрышей - Done!
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
			botMessage = statusLottery(m)
		// Установка параметров розыгрыша
		case "params":
			botMessage = paramsLottery(m)
		// Проверка списка участников розыгрыша - Done!
		case "check":
			botMessage = checkLottery(dg, m)
		// Старт розыгрыша
		case "start":
		// Справка по командам бота - Done!
		case "help":
			botMessage = "Справка по командам бота:\n\n" +
				"**list**\nПеречень списков розыгрышей\n" +
				"**create**\nСоздать новый список розыгрыша\n" +
				"**delete**\nУдалить существующий список розыгрышей\n" +
				"**show**\nПоказать список участников розыгрыша\n" +
				"**add**\nДобавить участника в список розыгрыша\n" +
				"**remove**\nУдалить участника из списка розыгрыша\n" +
				"**status**\nТекущие данные по розыгрышу\n" +
				"**params**\nУстановка параметров розыгрыша. Шаблон параметров: " +
				"!lottery params \"номер лотереи\"|\"выигрышное место\"|" +
				"\"количество победителей\"|\"получаемый приз\"\n" +
				"**check**\nПроверка списка участников розыгрыша\n" +
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
	files, err := filepath.Glob(filePrefix + "-*.csv")
	if err != nil {
		log.Print("Произошла ошибка: %s", err)
		return ""
	}
	text := "Найдены следующие списки розыгрышей: " + strconv.Itoa(len(files)) + "\n"
	text += strings.Join(files, "\n")
	return text
}

func getFileLotteryName(number string) string {
	return filePrefix + "-" + number + ".csv"
}

func findLottery(lotteryNumber string) (string, error) {
	file, err := filepath.Glob(getFileLotteryName(lotteryNumber))
	if err != nil {
		return "", err
	}
	if len(file) == 0 {
		return "Лотерея с номером *" + lotteryNumber + "* не найдена. Проверьте название или посмотрите список доступных розыгрышей командой **list**\n", nil
	}
	return getFileLotteryName(lotteryNumber), nil
}

func checkLottery(dg *discordgo.Session, m *discordgo.MessageCreate) string {
	// Получили список первых 1000 участников гильдии
	members, err := dg.GuildMembers(m.GuildID, "", 1000)
	errCheck("Ошибка при получении списка участников гильдии: ", err)
	var memberNicks []string
	for i := range members {
		if !members[i].User.Bot {
			if members[i].Nick != "" {
				memberNicks = append(memberNicks, members[i].Nick)
			} else {
				memberNicks = append(memberNicks, members[i].User.Username)
			}
		}
	}

	// Получаем список участников лотереи по именам из переданного названия файла
	loteryName, err := findLottery(strings.Fields(m.Content)[2])
	if err != nil {
		log.Print("Произошла ошибка: %s", err)
		return ""
	}

	csvFile, _ := os.Open(loteryName + ".csv")
	reader := csv.NewReader(bufio.NewReader(csvFile))
	var persons []string
	for {
		line, error := reader.Read()
		if error == io.EOF {
			break
		} else if error != nil {
			log.Fatal(error)
		}
		persons = append(persons, line[0])
	}
	csvFile.Close()

	//fmt.Println(persons)
	//fmt.Println(memberNicks)

	var isFind bool
	var str string
	for p := range persons {
		isFind = false
		for k := range memberNicks {
			if strings.HasPrefix(memberNicks[k], persons[p]) {
				// fmt.Printf("Найдено сопоставление %s - %s\n", persons[p], memberNicks[k])
				// str += "Найдено сопоставление участник розыгрыша - гильдиец: " + persons[p] + " - " + "memberNicks[k]" + "\n"
				isFind = true
				continue
			}
		}
		if !isFind {
			str += "Участник *" + persons[p] + "* не найден среди членов гильдии\n"
		}
	}

	//fmt.Printf("Участник: %s\n", members[0].Nick)
	if str == "" {
		str = "Все участники розыгрыша найдены среди гильдийцев\n"
	}
	return str
}

func statusLottery(m *discordgo.MessageCreate) string {
	loteryName, err := findLottery(strings.Fields(m.Content)[2])
	if err != nil {
		log.Print("Произошла ошибка: %s", err)
		return "Ошибка поиска списка участников\n"
	}
	return loteryName
}

func paramsLottery(m *discordgo.MessageCreate) string {
	var lottery Lottery
	params := strings.Fields(m.Content)
	loteryName, err := findLottery(params[2])
	if err != nil {
		log.Print("Произошла ошибка: %s", err)
		return "Ошибка поиска списка участников\n"
	}
	switch params[3] { // смотрим на тип задаваемой лотереи
	case "tournament":
		lottery.Type = TournamentLottery
	case "draw":
		lottery.Type = DrawLottery
	}
	return loteryName
}
