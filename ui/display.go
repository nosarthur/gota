package ui

import "fmt"

// ShowSummary displays all repos
func ShowSummary() {
	repos := GetRepos()
	fmt.Println(repos)
}
