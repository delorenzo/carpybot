package gobot

import (
	"bufio"
	"math"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/matrix-org/gomatrix"
)

type Question struct {
	Question string
	Score    int
	Answers  []string
}

type TriviaPlugin struct {
	Questions      []Question
	Client         *gomatrix.Client
	ActiveQuestion *Question
	ActiveTimer    *time.Timer
}

var score_re = regexp.MustCompile(`\(\$([0-9]+)\)`)

func round(f float64) int {
	return int(math.Floor(f + .5))
}

func parseQuestion(line string) Question {
	parts := strings.Split(line, "*")
	var score int
	if match := score_re.FindStringSubmatch(parts[0]); match != nil {
		score, _ = strconv.Atoi(match[1])
	} else {
		score = 200
	}
	return Question{Question: parts[0], Score: score, Answers: parts[1:]}
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if strings.ToLower(b) == strings.ToLower(a) {
			return true
		}
	}
	return false
}

func (tp *TriviaPlugin) Register(client *gomatrix.Client) {
	tp.Client = client
	syncer := client.Syncer.(*gomatrix.DefaultSyncer)
	syncer.OnEventType("m.room.message", tp.OnMessage)
}

func (tp *TriviaPlugin) OnMessage(ev *gomatrix.Event) {
	if ev.Content["body"] == "!trivia" {
		if tp.ActiveQuestion == nil {
			tp.NewQuestion(ev.RoomID)
			log.WithFields(log.Fields{
				"answer": tp.ActiveQuestion.Answers[0],
				"hint":   tp.HintActiveQuestion(0.25),
			}).Info("Testing hints")
		}
	} else if tp.ActiveQuestion != nil {
		tp.CheckAnswer(ev.RoomID, ev.Sender, ev.Content["body"].(string))
	}
}

func (tp *TriviaPlugin) NewQuestion(roomID string) {
	tp.ActiveQuestion = &tp.Questions[rand.Intn(len(tp.Questions))]
	log.WithFields(log.Fields{
		"answers": tp.ActiveQuestion.Answers,
	}).Info("Serving trivia question")
	tp.Client.SendText(roomID, tp.ActiveQuestion.Question)
	tp.ActiveTimer = time.NewTimer(time.Second * 25)
	go func() {
		<-tp.ActiveTimer.C
		tp.Client.SendText(roomID, "Time's up. Correct answers: "+strings.Join(tp.ActiveQuestion.Answers, ", "))
		log.Info("Question timed out.")
		tp.ActiveQuestion = nil
	}()
}

func (tp *TriviaPlugin) CheckAnswer(roomID string, sender string, answer string) {
	if tp.ActiveQuestion != nil && stringInSlice(answer, tp.ActiveQuestion.Answers) {
		if inTime := tp.ActiveTimer.Stop(); inTime {
			log.WithFields(log.Fields{
				"roomID":  roomID,
				"sender":  sender,
				"message": answer,
				"answers": tp.ActiveQuestion.Answers,
			}).Info("Correct answer provided.")
			tp.Client.SendText(roomID, sender+" is correct!")
			tp.ActiveQuestion = nil
		}
	}
}

func (tp *TriviaPlugin) HintActiveQuestion(frac float64) string {
	if tp.ActiveQuestion == nil {
		return ""
	}
	answer := tp.ActiveQuestion.Answers[0]
	hint := []byte(answer)
	num_letters := 0
	for i, char := range answer {
		if char != ' ' {
			hint[i] = '*'
			num_letters += 1
		}
	}
	perm := rand.Perm(len(answer))
	num_to_hint := round(float64(num_letters) * frac)
	if num_to_hint == 0 {
		num_to_hint = 1
	}
	hinted := 0
	for _, i := range perm {
		hint[i] = answer[i]
		hinted += 1
		if hinted >= num_to_hint {
			break
		}
	}
	return string(hint)
}

func NewTriviaPlugin() *TriviaPlugin {
	file, err := os.Open("jeopardy_s32.txt")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	questions := make([]Question, 0)
	for scanner.Scan() {
		questions = append(questions, parseQuestion(scanner.Text()))
	}
	return &TriviaPlugin{Questions: questions}
}
