# Quizzor [WIP]: Web app for playing quizzes together

This web app takes quizzes in the form of JSON files and serves them in a web UI, allowing you and your friends to play custom made quizzes.
Currently, available quizzes must be stored in the folder called ´quizzes´.
I plan to put paths to all imported data (quizzes and stylesheet) in environment variables defined in the docker compose file but I have not gotten around to it.

## Example Quiz Structure

```json
{
	"name": "The king of Omashu", //Name that will appear in the web UI
	"global_timer_s": -1, //Not yet implemented
	"questions": [
		{
    		"text": "What is my name?",
    		"timer_s": 30, // Not yet implemented
			"correct_answer": "Bumi",
    		"options": [
				"Ozai",
				"Bumi",
				"Gyatso",
				"Bah"
    		]
		}
	]
}
```

## Known Issues

1. The available quizzes are currently autocompletes for a text input and do not appear correctly on mobile.

2. The styling is currently quite hacked together and can lead main box to assume weird aspect ratios.
