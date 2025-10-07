package service

import (
	"log"
	"os/exec"
	"strings"
	"text/template"
)

var commentFuncMap template.FuncMap

func InitCommentFuncMap(characteristicsFile string) {
	// Define the FuncMap that will ONLY be available to the comment templates.
	commentFuncMap = template.FuncMap{
		"getDefaultCharacteristics": func(input string) string {
			out, err := exec.Command("bash", "-c", "jq \"."+input+"\" "+characteristicsFile).Output()
			if err != nil {
				log.Printf("Error executing jq command: %v", err)
				return ""
			}

			return string(out)
		},
	}
}

// ProcessComment takes a review, and parses and executes its comment field
// using the dedicated commentTemplate instance. This is the function that
// will be called from the main reviews.html template.
func ProcessComment(review Review) {
	if review.Comment == nil || *review.Comment == "" {
		return
	}

	// For thread safety, clone the base template before parsing and executing.
	tmpl, err := template.New("Review").Funcs(commentFuncMap).Parse(*review.Comment)
	if err != nil {
		log.Printf("Error parsing comment sub-template: %v", err)
		*review.Comment = "[Error: Invalid Comment Template]"
		return
	}

	var processed strings.Builder
	// Execute the now-parsed comment template with the review as its data context.
	if err := tmpl.Execute(&processed, review); err != nil {
		log.Printf("Error executing comment sub-template: %v", err)
		*review.Comment = "[Error: Could not render comment]"
		return
	}

	// Return the final, processed HTML.
	*review.Comment = processed.String()
}
