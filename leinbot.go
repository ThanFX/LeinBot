package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"errors"
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
	Members int64  // Количество победителей на этом месте
	Prize   string // Приз
}

type Lottery struct {
	Type        LotteryType
	Tournaments map[int64]Tournament
}

var (
	commandPrefix string
	filePrefix    string
	botID         string
	keys          Keys
	lotteries     map[int64]Lottery
)

func errCheck(msg string, err error) {
	if err != nil {
		fmt.Printf("%s: %+v\n", msg, err)
		panic(err)
	}
}

func main() {
	lotteries = make(map[int64]Lottery)

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
	} else if message.Content == "тупой бот" {
		discord.ChannelMessageSend(message.ChannelID, "сам такой :P\n")
	}

	//fmt.Printf("Message: \"%+v\" from: %s\n", message.Content, message.Author.Username)
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
		// Текущие данные по розыгрышу - Done!
		case "status":
			botMessage = statusLottery(m)
		// Установка параметров розыгрыша - Done!
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
				"**list**\nПеречень списков розыгрышей\n\n" +
				"**create**\nСоздать новый список розыгрыша\n" +
				"**delete**\nУдалить существующий список розыгрышей\n" +
				"**show**\nПоказать список участников розыгрыша\n" +
				"**add**\nДобавить участника в список розыгрыша\n" +
				"**remove**\nУдалить участника из списка розыгрыша\n\n" +

				"**status**\nТекущие данные по розыгрышу. Шаблон параметров: !lottery status \"номер лотереи\". Например !lottery status 1\n\n" +

				"**params**\nУстановка параметров розыгрыша. Шаблон параметров: " +
				"!lottery params \"номер лотереи\" \"тип лотереи\" \"выигрышное место\"|" +
				"\"количество победителей\"|\"получаемый приз\". Например !lottery params 1 tournament 3|5|50к золота\n" +
				"Для лотереи №1 установить тип \"турнир (по призовым местам)\" и задать для 3-го места 5 победителей, каждый получит по 50к золота\n\n" +

				"**check**\nПроверка списка участников розыгрыша. Шаблон параметров: !lottery check \"номер лотереи\". Например !lottery check 1\n\n" +

				"**start**\nСтарт розыгрыша\n" +
				"**help**\nСправка по командам бота\n"
		}
		if botMessage != "" {
			dg.ChannelMessageSend(m.ChannelID, botMessage)
		}
	} else {
		//dg.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Hello, %s!", m.Author.Username))
	}
}

func listLottery() string {
	files, err := filepath.Glob(filePrefix + "-*.csv")
	if err != nil {
		log.Print("Произошла ошибка: %s", err)
		return ""
	}
	text := "Найдено розыгрышей: " + strconv.Itoa(len(files)) + "\n"
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
	fmt.Printf("%+v", len(file))
	if len(file) == 0 {
		return "", errors.New("Файл не найден\n")
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
	lotteryNumber := strings.Fields(m.Content)[2]
	fileName, err := findLottery(lotteryNumber)
	if err != nil {
		log.Printf("Произошла ошибка: %s", err)
		return "Лотерея с номером *" + lotteryNumber + "* не найдена. Проверьте название или посмотрите список доступных розыгрышей командой **list**\n"
	}

	csvFile, err := os.Open(fileName)
	if err != nil {
		return "Ошибка открытия файла со списком участников гильдии\n"
	}
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
	var count int
	for p := range persons {
		isFind = false
		for k := range memberNicks {
			if strings.HasPrefix(memberNicks[k], persons[p]) {
				// fmt.Printf("Найдено сопоставление %s - %s\n", persons[p], memberNicks[k])
				// str += "Найдено сопоставление участник розыгрыша - гильдиец: " + persons[p] + " - " + "memberNicks[k]" + "\n"
				isFind = true
				count++
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
	str += "Всего в лотерее принимают участие: **" + strconv.Itoa(count) + "**\n"
	return str
}

func statusLottery(m *discordgo.MessageCreate) string {
	fields := strings.Fields(m.Content)
	if len(fields) != 3 {
		return "Ошибка команды, наберите **!lottery help** для вызова справки"
	}
	num := strings.Fields(m.Content)[2]
	_, err := findLottery(num)
	if err != nil {
		log.Print("Произошла ошибка: %s", err)
		return "Ошибка поиска списка участников\n"
	}
	lotNum, err := strconv.ParseInt(num, 10, 64)
	if err != nil {
		log.Print("Произошла ошибка: %s", err)
		return "Ошибочно указан номер лотереи\n"
	}

	s := "Параметры лотереи №**" + strconv.Itoa(int(lotNum)) + "** не заданы\n"
	for k, v := range lotteries {
		// Ищем выбранную лотерею
		if k == lotNum {
			s = "Текущий статус лотереи №**" + strconv.Itoa(int(k)) + "**:\n"
			switch v.Type {
			case 0:
				s += "Тип лотереи: выигрышные места\n"
			case 1:
				s += "Тип лотереи: розыгрыш билетов\n"
			}
			for i, pos := range v.Tournaments {
				s += "Победителей с местом №**" + strconv.Itoa(int(i)) + "** - **" + strconv.Itoa(int(pos.Members)) +
					"**, приз - **" + pos.Prize + "**\n"
			}
		}
	}
	return s
}

// !lottery params 1 tournament 3|5|50к золота
func paramsLottery(m *discordgo.MessageCreate) string {
	var lottery Lottery
	lottery.Tournaments = make(map[int64]Tournament)
	var tour Tournament
	data := strings.Fields(m.Content)
	_, err := findLottery(data[2])
	if err != nil {
		log.Print("Произошла ошибка: %s", err)
		return "Ошибка поиска списка участников\n"
	}
	lotNum, err := strconv.ParseInt(data[2], 10, 64)
	if err != nil {
		log.Print("Произошла ошибка: %s", err)
		return "Ошибочно указан номер лотереи\n"
	}
	// Ищем, есть ли уже параметры для этой лотереи
	for k, v := range lotteries {
		if k == lotNum {
			lottery = v
			break
		}
	}

	// Собираем в одну строку все параметры места (в описании выигрыша могут быть пробелы которые рассплитило ранее)
	lotteryParam := strings.Join(data[4:], " ")
	// И теперь сплитим их по другому разделителю
	params := strings.Split(lotteryParam, "|")

	lotteryType := data[3]
	switch lotteryType { // смотрим на тип задаваемой лотереи
	case "tournament":
		lottery.Type = TournamentLottery
		num, err := strconv.ParseInt(params[1], 10, 64)
		if err != nil {
			log.Print("Произошла ошибка: %s", err)
			return "Ошибочно указано количество участников\n"
		}
		tour.Members = num
		tour.Prize = params[2]
		num, err = strconv.ParseInt(params[0], 10, 64)
		if err != nil {
			log.Print("Произошла ошибка: %s", err)
			return "Ошибочно указано призовое место\n"
		}
		lottery.Tournaments[num] = tour
		num, err = strconv.ParseInt(data[2], 10, 64)
		if err != nil {
			log.Print("Произошла ошибка: %s", err)
			return "Ошибочно указан номер лотереи\n"
		}
		lotteries[num] = lottery
	case "draw":
		lottery.Type = DrawLottery
		return "Лотерея этого типа пока недоступна\n"
	}
	fmt.Printf("%+v\n", lotteries)
	return "Для лотереи №" + data[2] + " успешно заданы параметры розыгрыша\n"
}
