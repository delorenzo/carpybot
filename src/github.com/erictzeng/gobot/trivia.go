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
	"unicode"

	log "github.com/Sirupsen/logrus"
	"github.com/matrix-org/gomatrix"
	"github.com/texttheater/golang-levenshtein/levenshtein"
)

type Question struct {
	Question string
	Score    int
	Answers  []string
}

type TriviaPlugin struct {
	Questions      []Question
	QuestionOrder  []int
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

var levenshteinOptions levenshtein.Options = levenshtein.Options{
	InsCost: 1,
	DelCost: 1,
	SubCost: 1,
	Matches: levenshtein.DefaultOptions.Matches,
}

func stringInSlice(a string, list []string) bool {
	guess := []rune(strings.ToLower(a))
	for _, b := range list {
		correct := []rune(strings.ToLower(b))
		num_runes := len(correct)
		max_distance := num_runes / 8
		if levenshtein.DistanceForStrings(guess, correct, levenshteinOptions) <= max_distance {
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
	if ev.Content["body"] == "!trivia" && tp.ActiveQuestion == nil {
		tp.NewQuestion(ev.RoomID)
	} else if tp.ActiveQuestion != nil {
		tp.CheckAnswer(ev.RoomID, ev.Sender, ev.Content["body"].(string))
	}
}

func (tp *TriviaPlugin) SampleQuestion() *Question {
	if len(tp.QuestionOrder) == 0 {
		tp.QuestionOrder = rand.Perm(len(tp.Questions))
	}
	i := tp.QuestionOrder[0]
	tp.QuestionOrder = tp.QuestionOrder[1:]
	return &tp.Questions[i]
}

func (tp *TriviaPlugin) NewQuestion(roomID string) {
	tp.ActiveQuestion = tp.SampleQuestion()
	log.WithFields(log.Fields{
		"answers": tp.ActiveQuestion.Answers,
	}).Info("Serving trivia question")
	tp.Client.SendText(roomID, tp.ActiveQuestion.Question)
	tp.ActiveTimer = time.NewTimer(time.Second * 13)
	go func() {
		<-tp.ActiveTimer.C
		tp.Client.SendText(roomID, "Hint: "+tp.HintActiveQuestion(0.25))
		tp.ActiveTimer = time.NewTimer(time.Second * 13)
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
			tp.Client.SendText(roomID, sender+" is correct! Answers: "+strings.Join(tp.ActiveQuestion.Answers, ", "))
			tp.ActiveQuestion = nil
		}
	}
}

func (tp *TriviaPlugin) HintActiveQuestion(frac float64) string {
	if tp.ActiveQuestion == nil {
		return ""
	}
	answer := []rune(tp.ActiveQuestion.Answers[0])
	hint := make([]rune, len(answer))
	num_letters := 0
	for i, char := range answer {
		if unicode.IsLetter(char) {
			hint[i] = '*'
			num_letters += 1
		} else {
			hint[i] = char
		}
	}
	perm := rand.Perm(len(answer))
	num_to_hint := round(float64(num_letters) * frac)
	if num_to_hint == 0 {
		num_to_hint = 1
	}
	hinted := 0
	for _, i := range perm {
		if unicode.IsLetter(answer[i]) {
		    hint[i] = answer[i]
		    hinted += 1
		    if hinted >= num_to_hint {
			    break
		    }
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
