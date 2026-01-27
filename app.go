package main

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strings"
	"time"
)

type Code string

type Question struct {
	question string
	timer_s  int //time in seconds
	options  []string
}

type Player struct {
	name            string
	correct_answers []bool
}

type Quiz struct {
	Name            string
	Code            Code
	Global_timer_s  int //override question timer
	Active_question int
	Questions       []Question
	Players         []Player
}

var active_quizzes map[Code]Quiz = make(map[Code]Quiz)

const QUIZZES_DIR string = "./quizzes"
const CODE_LEN = 6
const CODE_CHARS = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

func (q Quiz) add_player(p Player) {
	q.Players = append(q.Players[:], p)
}

func make_code() Code {
	seed := time.Now().Local().UnixMicro()
	code_arr := make([]byte, CODE_LEN)
	code_arr[0] = CODE_CHARS[seed%int64(len(CODE_CHARS))]
	for i := int64(1); i < CODE_LEN; i++ {
		code_arr[i] = CODE_CHARS[(int64(code_arr[i-1])*seed^seed>>i)%int64(len(CODE_CHARS))]
	}
	for _, exists := active_quizzes[Code(string(code_arr))]; exists; {
		code_arr[0] = CODE_CHARS[(int64(code_arr[0])^seed)%int64(len(CODE_CHARS))]
	}
	return Code(string(code_arr))
}

func read_quizzes() []string {
	quizzes_dir, err := os.ReadDir(QUIZZES_DIR)
	if err != nil {
		panic("Panic! Could not read quiz directory")
	}
	var quiz_names []string
	for _, q := range quizzes_dir {
		var s string = q.Name()
		s = strings.ReplaceAll(s, ".json", "")
		quiz_names = append(quiz_names, s)
		print("appended quiz: ", s, "\n")
	}

	return quiz_names
}

func main() {
	quiz_names := read_quizzes()
	start_tmpl := template.Must(template.ParseFiles("main.html"))
	quiz_tmpl := template.Must(template.ParseFiles("quiz.html"))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Show start page
		if r.Method != http.MethodPost {
			//fmt.Fprint(w, r.Host)
			start_tmpl.Execute(w, struct{ Quizzes []string }{quiz_names[:]})
			return
		}

		input := r.FormValue("input")
		name := r.FormValue("name")
		new_quiz, exists := active_quizzes[Code(input)]

		if !exists {
			new_quiz = Quiz{
				input,
				make_code(),
				-1,
				0,
				make([]Question, 0),
				make([]Player, 0),
			}
		}

		new_quiz.add_player(Player{name, make([]bool, 0)})
		active_quizzes[new_quiz.Code] = new_quiz

		fmt.Fprint(os.Stdout, "queried quiz: ", input, "\n")
		quiz_tmpl.Execute(w, struct{ Quiz Quiz }{new_quiz})
	})

	http.HandleFunc("/quiz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			quiz_tmpl.Execute(w, struct{ Quiz string }{r.URL.Path})
		}
	})

	http.ListenAndServe(":5656", nil)
}
