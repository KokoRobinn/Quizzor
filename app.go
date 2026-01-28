package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"maps"
	"net/http"
	"os"
	"path"
	"slices"
	"strings"
	"sync"
	"time"
)

type Code string

type Question struct {
	Text    string   `json:"text"`
	Timer_s int      `json:"timer_s"` //time in seconds
	Options []string `json:"options"`
}

type Player struct {
	Name            string
	Correct_answers []bool
}

type Quiz struct {
	Name           string `json:"name"`
	Code           Code
	Broker         *Broker
	Global_timer_s int `json:"global_timer_s"` //overriden by question timer
	Question_index int
	Questions      []Question `json:"questions"`
	Players        []Player
}

var available_quizzes map[string]Quiz
var active_quizzes map[Code]Quiz = make(map[Code]Quiz)

const QUIZZES_DIR string = "./quizzes"
const CODE_LEN = 6
const CODE_CHARS = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

type Broker struct {
	clients map[chan string]bool
	lock    sync.Mutex
}

func newBroker() *Broker {
	return &Broker{clients: make(map[chan string]bool)}
}

func (b *Broker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	msgChan := make(chan string)
	b.lock.Lock()
	b.clients[msgChan] = true
	b.lock.Unlock()

	defer func() {
		b.lock.Lock()
		delete(b.clients, msgChan)
		b.lock.Unlock()
		close(msgChan)
	}()

	for msg := range msgChan {
		fmt.Fprintf(w, "data: %s\n\n", msg)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
}

func (b *Broker) Broadcast(msg string) {
	b.lock.Lock()
	defer b.lock.Unlock()
	for client := range b.clients {
		client <- msg
	}
}

func (q *Quiz) Add_player(p Player) {
	q.Players = append(q.Players, p)
	fmt.Fprint(os.Stdout, "added player: ", p.Name, "\n", "total players for ", q.Code, ":\n")
	for _, plyr := range q.Players {
		fmt.Fprint(os.Stdout, "\t", plyr.Name, "\n")
	}
}

func make_code() Code {
	seed := time.Now().Local().UnixMicro()
	code_arr := make([]byte, CODE_LEN)
	code_arr[0] = CODE_CHARS[seed%int64(len(CODE_CHARS))]
	for i := int64(1); i < CODE_LEN; i++ {
		code_arr[i] = CODE_CHARS[(int64(code_arr[i-1])*seed^seed>>i)%int64(len(CODE_CHARS))]
	}
	//TODO: make this more robust
	for _, exists := active_quizzes[Code(string(code_arr))]; exists; {
		code_arr[0] = CODE_CHARS[(int64(code_arr[0])^seed)%int64(len(CODE_CHARS))]
	}
	return Code(string(code_arr))
}

func parse_quizzes() map[string]Quiz {
	quizzes_dir, err := os.ReadDir(QUIZZES_DIR)
	if err != nil {
		fmt.Println(err.Error())
	}
	var quizzes map[string]Quiz = make(map[string]Quiz)
	for _, q := range quizzes_dir {
		var s string = q.Name()
		s = strings.ReplaceAll(s, ".json", "")

		quiz_json_file, err := os.Open(path.Join(QUIZZES_DIR, q.Name()))
		if err != nil {
			fmt.Println(err.Error())
		}
		quiz := Quiz{}
		jsonParser := json.NewDecoder(quiz_json_file)
		if err = jsonParser.Decode(&quiz); err != nil {
			fmt.Println(err.Error())
		}
		fmt.Println(quiz)
		quizzes[s] = quiz
		print("appended quiz: ", s, "\n")
		quiz_json_file.Close()
	}

	return quizzes
}

func main() {
	available_quizzes = parse_quizzes()
	start_tmpl := template.Must(template.ParseFiles("main.html"))
	quiz_tmpl := template.Must(template.ParseFiles("quiz.html"))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Show start page
		if r.Method != http.MethodPost {
			//fmt.Fprint(w, r.Host)
			start_tmpl.Execute(w, struct{ Quizzes []string }{slices.Collect(maps.Keys(available_quizzes))})
			return
		}

		input := r.FormValue("input")
		player_name := r.FormValue("name")
		new_quiz, active := active_quizzes[Code(input)]
		if !active {
			available := true
			new_quiz, available = available_quizzes[input]
			fmt.Println(new_quiz)
			if !available {
				//Go home
				start_tmpl.Execute(w, struct{ Quizzes []string }{slices.Collect(maps.Keys(available_quizzes))})
				return
			}
			new_quiz.Code = make_code()
			new_quiz.Broker = newBroker()
			new_quiz.Question_index = 0
			new_quiz.Players = make([]Player, 0)
		}

		new_quiz.Add_player(Player{player_name, make([]bool, 0)})
		active_quizzes[new_quiz.Code] = new_quiz

		fmt.Fprint(os.Stdout, "queried quiz: ", input, "\n", new_quiz, "\n")
		quiz_tmpl.Execute(w, struct{ Quiz Quiz }{new_quiz})
	})

	http.HandleFunc("/quiz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			quiz_tmpl.Execute(w, struct{ Quiz string }{r.URL.Path})
		}
	})

	http.ListenAndServe(":5656", nil)
}
