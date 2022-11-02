package generator

import (
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/internal/message"
)


func askQuestion(question string, required bool) (answer string) {
	prompt := &survey.Input{
		Message: fmt.Sprint(question),
	}
	var err error
	if required {
		err = survey.AskOne(prompt, &answer, survey.WithValidator(survey.Required))
	} else {
		err = survey.AskOne(prompt, &answer)
	}
	if err != nil {
		message.Fatal("", err.Error())
	}
	return answer
}

func isDir(path string) bool {
	pathData, err := os.Stat(path)
	if err != nil {
		message.Fatal(err, "Error stat-ing path")
	}
	return pathData.IsDir()
}