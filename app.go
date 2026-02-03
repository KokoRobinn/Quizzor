package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"maps"
	"net/http"
	"os"
	"path"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"
)

type Code string

type Question struct {
	Text           string   `json:"text"`
	Timer_s        int      `json:"timer_s"` //time in seconds
	Correct_answer string   `json:"correct_answer"`
	Options        []string `json:"options"`
}

type Answer struct {
	Answer  string
	Correct bool
}

type Player struct {
	Name           string
	Current_answer string
	Answers        []Answer
}

type Quiz struct {
	Name           string `json:"name"`
	Code           Code
	Broker         *Broker
	Global_timer_s int `json:"global_timer_s"` //overriden by question timer
	Question_index int
	Questions      []Question `json:"questions"`
	Players        map[string]*Player
}

var available_quizzes map[string]Quiz
var active_quizzes map[Code]*Quiz = make(map[Code]*Quiz)

const QUIZZES_DIR string = "./quizzes"
const CODE_LEN = 6
const CODE_CHARS = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
const PLAYER_HASH_LEN = 10
const PLAYER_HASH_CHARS = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

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
	q.Players[p.Name] = &p
	fmt.Fprintln(os.Stdout, "Added player", p.Name, "to", q.Code, "\n\ttotal players:")
	for _, plyr := range q.Players {
		fmt.Fprint(os.Stdout, "\t\t", plyr.Name, "\n")
	}
	q.Broker.Broadcast("update")
}

func (q *Quiz) Next_question() {
	for _, p := range q.Players {
		if q.Question_index >= 0 {
			is_correct := p.Current_answer == q.Questions[q.Question_index].Correct_answer
			p.Answers[q.Question_index] = Answer{p.Current_answer, is_correct}
			fmt.Fprintln(os.Stdout, p.Answers)
		}
		p.Current_answer = ""
	}
	q.Question_index += 1
	q.Broker.Broadcast("update")
	fmt.Fprintln(os.Stdout, "Incremented question index!\n\t", q)
}

func (q Quiz) Instantiate() *Quiz {
	val := reflect.ValueOf(q)

	// Create a new pointer to the struct type
	ptr := reflect.New(val.Type())

	// Set the value of the pointer to the original struct's value
	ptr.Elem().Set(val)
	return ptr.Interface().(*Quiz)
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

func make_player_hash(name string) string {
	seed := time.Now().Local().UnixMicro()
	player_hash := make([]byte, PLAYER_HASH_LEN)
	player_hash[0] = PLAYER_HASH_CHARS[seed%int64(len(PLAYER_HASH_CHARS))]
	for i := int64(1); i < CODE_LEN; i++ {
		player_hash[i] = CODE_CHARS[(int64(player_hash[i-1])*seed^int64(name[i%int64(len(name))])>>i)%int64(len(CODE_CHARS))]
	}
	//TODO: make this more robust
	for _, exists := active_quizzes[Code(string(player_hash))]; exists; {
		player_hash[0] = CODE_CHARS[(int64(player_hash[0])^seed)%int64(len(CODE_CHARS))]
	}
	return string(player_hash)
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
		quizzes[s] = quiz
		fmt.Fprintln(os.Stdout, "Appended quiz:", s)
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
			start_tmpl.Execute(w, struct{ Quizzes []string }{slices.Collect(maps.Keys(available_quizzes))})
			return
		}

		quiz_name := r.FormValue("quiz_name")
		player_name := r.FormValue("name")
		new_quiz, active := active_quizzes[Code(quiz_name)]
		fmt.Fprintln(os.Stdout, "Queried quiz:", quiz_name)
		if !active {
			available := true
			src_quiz, available := available_quizzes[quiz_name]
			new_quiz = src_quiz.Instantiate()
			if !available {
				//Go home
				start_tmpl.Execute(w, struct{ Quizzes []string }{slices.Collect(maps.Keys(available_quizzes))})
				return
			}
			new_quiz.Code = make_code()
			new_quiz.Broker = newBroker()
			new_quiz.Question_index = -1
			new_quiz.Players = make(map[string]*Player, 0)
			http.HandleFunc("/events/"+string(new_quiz.Code), new_quiz.Broker.ServeHTTP)
			fmt.Fprintln(os.Stdout, "Quiz", quiz_name, "not active! Instantiating new quiz with code:", new_quiz.Code)
		}

		new_quiz.Add_player(Player{player_name, "", make([]Answer, len(new_quiz.Questions))})
		active_quizzes[new_quiz.Code] = new_quiz

		player_cookie := http.Cookie{
			Name:     "player",
			Value:    player_name,
			Path:     "/",
			MaxAge:   3600,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteDefaultMode,
		}

		http.SetCookie(w, &player_cookie)
		http.Redirect(w, r, "/quiz/"+string(new_quiz.Code), http.StatusSeeOther)
	})

	http.HandleFunc("/quiz/", func(w http.ResponseWriter, r *http.Request) {
		player_cookie, err := r.Cookie("player")
		http.SetCookie(w, player_cookie)
		if err != nil {
			fmt.Fprintln(os.Stdout, "Failed to get player_cookie:", err)
		}

		quiz_code := Code(strings.ReplaceAll(r.URL.Path, "/quiz/", ""))
		q := active_quizzes[quiz_code]
		p := q.Players[player_cookie.Value]
		if r.Method == http.MethodPost {
			// Next question
			if r.FormValue("progress") != "" {
				if q.Question_index == len(q.Questions) {
					http.Redirect(w, r, "/", http.StatusSeeOther)
					return
				}
				q.Next_question()
			}

			// Answer submission
			if a := r.FormValue("answer"); a != "" {
				fmt.Fprintln(os.Stdout, "Player", p.Name, "answered", a, "in quiz", q.Name, ":", q.Question_index)
				p.Current_answer = a
			}
		}
		quiz_tmpl.Execute(w, struct {
			Quiz   Quiz
			Player Player
		}{*q, *p})
	})

	http.ListenAndServe(":5656", nil)
}
