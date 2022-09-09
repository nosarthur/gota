package ui

import (
	"encoding/csv"
	"log"
	"os"
	"path/filepath"
)

type Repo struct {
	Path  string
	Name  string
	Flags string // options that immediately follow the git command
}

func (r Repo) String() string {
	return r.Name
}

func (r *Repo) Unmarshal(data []string) error {
	r.Path = data[0]
	r.Name = data[1]
	return nil
}

func GetRepos() []Repo {
	home, err := os.UserHomeDir()
	if err != nil {
		// TODO: what to do?
		log.Fatal(err)
	}
	configPath := filepath.Join(home, ".config", "gita", "repos.csv")
	data, err := loadConfig(configPath)
	repos := make([]Repo, len(data))

	for i := range repos {
		repos[i].Unmarshal(data[i])
	}

	return repos
}

func loadConfig(fname string) ([][]string, error) {
	f, err := os.Open(fname)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	// read csv values using csv.Reader
	csvReader := csv.NewReader(f)
	return csvReader.ReadAll()

}
