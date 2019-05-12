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
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

type Keys struct {
	Token    string   `json: "token"`
	Admins   []string `json: "admins"`
	Guilds   []string `json: "guilds"`
	Channels []string `json: "channels"`
}

type DgNick struct {
	Nick   string
	DgUser *discordgo.Member
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
	winPhrases    = []string{
		"(Незнакомая аргонианка в первом ряду недвусмысленно строит вам глазки. Все любят победителей)",
		"(Жидкие аплодисменты. Ради приличия)",
		"(Зал оглушительно воет! Все безумно рады вашей победе)",
		"(Зловещая тишина)",
		"(Подозрительные каджиты с последнего ряда загадочно перешептываются и потирают лапы, приметив цель для легкого обогащения)",
		"(Какой-то завистник метнул в вас переспевшим помидором. Вам не успеть увернуться)",
		"(Вы рыдаете от счастья. Пытаетесь говорить благодарности родне, друзьям, гуарам и скампам, но ведущий бесцеремонно прогоняет вас со сцены)",
		"(Вы злобно ухмыляетесь. Именно эту победу вам и обещал Молаг Бал в обмен на вашу душу)",
		"(Зал угрюмо перешептывается. Ведь все знают, что вы подкупили судей)",
		"(Ваши друзья срываются с мест, направляясь в ближайшую таверну. Зачем вы обещали им проставиться?)",
		"(Надо же, аргонианская смесь из древесных личинок, жабьих глаз и лепешек гуара действительно приносит победу!)",
		"(Вы падаете в обморок от избытка чувств)",
		"(Вы широко улыбаетесь своей чудесной улыбкой во все 54 зуба)",
		"(Толпа подхватывает вас на руки. Вас качают и подбрасывают вверх, крича \"Ура\". И даже почти всегда ловят)",
		"(Симпатичный босмер ласково гладит вас по плечу. Вы ему понравились)",
		"(Вы делаете сальто назад, ловко приземляясь на пятую точку. Публика в восторге)",
		"(Вы слышите шепот ведущего \"Как и договаривались. 50 на 50\". Он хитро подмигивает)",
		"(Зрители молча встают со своих мест и выходят из зала)",
		"(Вы так торопитесь на сцену за наградой, что по пути наступаете на край платья одной из эльфиек, делая представление еще более увлекательным)",
		"(Зал поделился на тех, кто за вас рад, и на ваших завистников. Драка неизбежна)",
	}
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
	text := strings.Fields(strings.TrimSpace(m.Content))
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
			botMessage = startLottery(dg, m)
		// Справка по командам бота - Done!
		case "help":
			botMessage = "Справка по командам бота:\n\n" +
				"**list**\nПеречень списков розыгрышей\n\n" +
				//"**create**\nСоздать новый список розыгрыша\n" +
				//"**delete**\nУдалить существующий список розыгрышей\n" +
				//"**show**\nПоказать список участников розыгрыша\n" +
				//"**add**\nДобавить участника в список розыгрыша\n" +
				//"**remove**\nУдалить участника из списка розыгрыша\n\n" +

				"**status**\nТекущие данные по розыгрышу. Шаблон параметров: !lottery status \"номер лотереи\". Например !lottery status 1\n\n" +

				"**params**\nУстановка параметров розыгрыша. Шаблон параметров: " +
				"!lottery params \"номер лотереи\" \"тип лотереи\" \"выигрышное место\"|" +
				"\"количество победителей\"|\"получаемый приз\". Например !lottery params 1 tournament 3|5|50к золота\n" +
				"Для лотереи №1 установить тип \"турнир (по призовым местам)\" и задать для 3-го места 5 победителей, каждый получит по 50к золота\n\n" +

				"**check**\nПроверка параметров розыгрыша. Шаблон параметров: !lottery check \"номер лотереи\". Например !lottery check 1\n\n" +

				"**start**\nСтарт розыгрыша. Шаблон параметров: !lottery start \"номер лотереи\". Например !lottery start 1\n\n" +
				"**help**\nСправка по командам бота\n"
		}
		if botMessage != "" {
			dg.ChannelMessageSend(m.ChannelID, botMessage)
		}
	} else {
		//dg.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Hello, %s!", m.Author.Username))
	}
}

// Возвращаем список всех найденных по префиксу файлов лотереи
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

// Возвращаем название файла лотерии по номеру и префиксу
func getFileLotteryName(number string) string {
	return filePrefix + "-" + number + ".csv"
}

// Проверяем наличие файла лотереи с указанным номером и возвращаем название файла, если такой файл нашёлся
func findLotteryFile(lotteryNumber string) (string, error) {
	file, err := filepath.Glob(getFileLotteryName(lotteryNumber))
	if err != nil {
		return "", err
	}
	//fmt.Printf("%+v", len(file))
	if len(file) == 0 {
		return "", errors.New("Файл не найден\n")
	}
	return getFileLotteryName(lotteryNumber), nil
}

// Проверяем корректность переданного в параметрах номера лотереи
func getLotteryNumber(st string) (int64, error) {
	data := strings.Fields(strings.TrimSpace(st))
	if len(data) < 3 {
		return 0, errors.New("Не указан номер лотереи\n")
	}
	lotteryNumber := strings.Fields(strings.TrimSpace(st))[2]
	lotNum, err := strconv.ParseInt(lotteryNumber, 10, 64)
	if err != nil {
		return 0, errors.New("Ошибочно указан номер лотереи\n")
	}
	return lotNum, nil
}

// Проверяем корректность переданного в параметрах номера лотереи, наличие файла с соответствующим именем и возвращаем название файла, если всё корректно
func getLotteryFile(st string) (string, error) {
	num, err := getLotteryNumber(st)
	if err != nil {
		return "", err
	}
	fileName, err := findLotteryFile(strconv.FormatInt(num, 10))
	if err != nil {
		return "", errors.New("Лотерея с номером *" + strconv.FormatInt(num, 10) + "* не найдена. Проверьте название или посмотрите список доступных розыгрышей командой **list**\n")
	}
	return fileName, nil
}

// Возвращаем массив структур первых 1000 участников гильдии
func getDGUsers(dg *discordgo.Session, guildID string) ([]DgNick, error) {
	var memberNicks []DgNick
	members, err := dg.GuildMembers(guildID, "", 1000)
	if err != nil {
		return nil, errors.New("Ошибка при получении списка участников гильдии: " + err.Error() + "\n")
	}
	nick := DgNick{}
	for i := range members {
		if !members[i].User.Bot {
			if members[i].Nick != "" {
				nick.Nick = members[i].Nick
			} else {
				nick.Nick = members[i].User.Username
			}
			nick.DgUser = members[i]
			memberNicks = append(memberNicks, nick)
		}
	}
	return memberNicks, nil
}

// Возвращаем список ников участников лотереи из файла с номером лотереи
func getLotteryPersonsFromFile(fileName string) ([]string, error) {
	csvFile, err := os.Open(fileName)
	if err != nil {
		return nil, errors.New("Ошибка открытия файла со списком участников гильдии\n")
	}
	defer csvFile.Close()
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
	return persons, nil
}

// Возвращаем параметры лотереи (тип, места, количество победителей и приз для каждого места)
func statusLottery(m *discordgo.MessageCreate) string {
	lotNum, err := getLotteryNumber(m.Content)
	if err != nil {
		return err.Error()
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

func checkLottery(dg *discordgo.Session, m *discordgo.MessageCreate) string {
	memberNicks, err := getDGUsers(dg, m.GuildID)
	if err != nil {
		return err.Error()
	}

	// Получаем список участников лотереи по именам из переданного названия файла
	fileName, err := getLotteryFile(m.Content)
	if err != nil {
		return err.Error()
	}
	lotNum, _ := getLotteryNumber(m.Content)
	persons, err := getLotteryPersonsFromFile(fileName)
	if err != nil {
		return err.Error()
	}

	var isFind bool
	var str string
	var count int
	for p := range persons {
		isFind = false
		for k := range memberNicks {
			if strings.HasPrefix(memberNicks[k].Nick, persons[p]) {
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

	var countPrizeMembers int64
	countPrizeMembers = 0
	for k, v := range lotteries {
		//fmt.Println(342)
		// Ищем выбранную лотерею
		if k == lotNum {
			switch v.Type {
			case 0:
				for _, pos := range v.Tournaments {
					countPrizeMembers += pos.Members
				}
			case 1:

			}
		}
	}

	str += statusLottery(m)
	str += "Всего в лотерее призовых мест: **" + strconv.Itoa(int(countPrizeMembers)) + "**\n"
	if int(countPrizeMembers) > count {
		str += "\n**Внимание! Количество призовых мест превышает количество участников!!**\n"
	}
	if count > len(winPhrases) {
		str += "\n**Внимание! Количество призовых мест превышает количество фраз для победителей!!**\n"
	}
	return str
}

func paramsLottery(m *discordgo.MessageCreate) string {
	var lottery Lottery
	lottery.Tournaments = make(map[int64]Tournament)
	var tour Tournament
	data := strings.Fields(strings.TrimSpace(m.Content))
	lotNum, err := getLotteryNumber(m.Content)
	if err != nil {
		return err.Error()
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
	//fmt.Printf("%+v\n", lotteries)
	return "Для лотереи №" + data[2] + " успешно заданы параметры розыгрыша\n"
}

func startLottery(dg *discordgo.Session, m *discordgo.MessageCreate) string {
	memberNicks, err := getDGUsers(dg, m.GuildID)
	if err != nil {
		return err.Error()
	}

	// Получаем список участников лотереи по именам из переданного названия файла
	fileName, err := getLotteryFile(m.Content)
	if err != nil {
		return err.Error()
	}
	lotNum, _ := getLotteryNumber(m.Content)
	var s string

	switch lotteries[lotNum].Type {
	case TournamentLottery:
		persons, err := getLotteryPersonsFromFile(fileName)
		if err != nil {
			return err.Error()
		}

		rand.Seed(time.Now().UTC().UnixNano())
		maxTour := int64(len(lotteries[lotNum].Tournaments))
		var curTour = maxTour
		var winners []string
		for curTour > 0 {
			maxWin := lotteries[lotNum].Tournaments[curTour].Members
			curWin := maxWin
			for curWin > 0 {
				winnerNum := rand.Intn(len(persons))
				winners = append(winners, persons[winnerNum])
				persons = append(persons[:winnerNum], persons[winnerNum+1:]...)
				curWin--
			}
			curTour--
		}

		curWinPhrases := make([]string, len(winPhrases))
		copy(curWinPhrases, winPhrases)
		var cwp string
		for i := range winners {
			for _, v := range memberNicks {
				if strings.HasPrefix(v.Nick, winners[i]) {
					if len(curWinPhrases) > 0 {
						winPhraseNum := rand.Intn(len(curWinPhrases))
						cwp = curWinPhrases[winPhraseNum]
						curWinPhrases = append(curWinPhrases[:winPhraseNum], curWinPhrases[winPhraseNum+1:]...)
					} else {
						cwp = ""
					}
					s += "Победитель " + v.DgUser.Mention() + " " + cwp + "\n\n"
				}
			}
		}

	case DrawLottery:
	}
	return s
}

// !lottery params 1 tournament 1|5|50к золота
