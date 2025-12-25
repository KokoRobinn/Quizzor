package main

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strings"
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
	name            string
	global_timer_s  int //override question timer
	active_question int
	questions       []Question
	players         []Player
}

var active_quizzes map[Code]Quiz

const QUIZZES_DIR string = "./quizzes"

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

		fmt.Fprint(w, input)
		//tmpl.Execute(w, struct{ Success bool; File string }{ true, CODES_DIR + file })
	})

	http.HandleFunc("/quiz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			quiz_tmpl.Execute(w, nil)
		}
	})

	http.ListenAndServe(":5656", nil)
}
