package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jmcvetta/neoism"
)

type Game struct {
	db *neoism.Database
}

type room struct {
	description string
	exits       []string
	contents    []string
}

func NewGame(uri string) (game *Game, err error) {
	g := Game{}

	g.db, err = neoism.Connect(uri)
	if err != nil {
		return nil, err
	}

	return &g, nil
}

func NewRoom(d <-chan string, cs <-chan string) *room {
	var contents []string
	description := <-d

	for c := range cs {
		contents = append(contents, c)
	}
	return &room{
		description: description,
		contents:    contents,
	}
}

func (g Game) getRoomDescription(d chan<- string) {
	defer close(d)

	room := []struct {
		Description string `json:"r.description"`
	}{}

	cq := neoism.CypherQuery{
		Statement: "MATCH (:PLAYER)-[:IS_IN]-(r:ROOM) RETURN r.description",
		Result:    &room,
	}

	if err := g.db.Cypher(&cq); err != nil {
		// Let's suppose (yeah, I said that!) that it will not error for now
	}

	// The player will always be in a ROOM
	d <- room[0].Description
}

func (g Game) getRoomContentDescriptions(ds chan<- string) {
	defer close(ds)

	contents := []struct {
		Description string `json:"c.description"`
	}{}

	cq := neoism.CypherQuery{
		Statement: "MATCH (:PLAYER)-[:IS_IN]-(:ROOM)-[:CONTENTS]-(c) RETURN c.description",
		Result:    &contents,
	}

	if err := g.db.Cypher(&cq); err != nil {
		// DITTO
	}

	for _, c := range contents {
		ds <- c.Description
	}
}

func (g Game) move(d string) {
	cq := neoism.CypherQuery{
		Statement: `
          MATCH (p:PLAYER)-[i:IS_IN]-(:ROOM)-[m:MOVE]->(r:ROOM)
		  WHERE m.direction = {d}
		  DELETE i
		  CREATE (p)-[:IS_IN]->(r)
		`,
		Parameters: neoism.Props{"d": d},
	}

	if err := g.db.Cypher(&cq); err != nil {
		// DITTO
	}
}

func (g Game) getRoom() *room {
	d := make(chan string)
	go g.getRoomDescription(d)

	cs := make(chan string)
	go g.getRoomContentDescriptions(cs)

	return NewRoom(d, cs)
}

func (g Game) canMove(d string) bool {
	exits := []struct {
		Count int `json:"count(m)"`
	}{}

	cq := neoism.CypherQuery{
		Statement: `
		    MATCH (:PLAYER)-[:IS_IN]-(:ROOM)-[m:MOVE]->(:ROOM)
		    WHERE m.direction = {d}
		    RETURN count(m)
		`,
		Parameters: neoism.Props{"d": d},
		Result:     &exits,
	}

	if err := g.db.Cypher(&cq); err != nil {
		// DITTO
	}

	return exits[0].Count > 0
}

func (r room) describe() {
	fmt.Println(r.description)

	fmt.Printf("You see: ")
	if len(r.contents) != 0 {
		for d := range r.contents {
			fmt.Printf("\n- %s", d)
		}
	} else {
		fmt.Println("nothing.")
	}
}

func (g Game) loop() {
	reader := bufio.NewReader(os.Stdin)

	_cleanInput := func(input string) []string {
		input = strings.Replace(input, "\n", "", -1)
		input = strings.ToLower(input)
		return strings.Split(input, " ")
	}

	for {
		g.getRoom().describe()

		fmt.Printf("\nWhat do you want to do?: ")
		input, _ := reader.ReadString('\n')
		action := _cleanInput(input)

		switch action[0] {
		case "move":
			direction := ""
			if len(action) > 1 {
				direction = action[1]
			}
			if g.canMove(direction) {
				g.move(direction)
			} else {
				fmt.Println("I can't move there")
			}
		default:
			fmt.Println("I can't do that!")
		}

		fmt.Println()
	}
}

func main() {
	g, err := NewGame("http://neo4j:password@b2d:7474/db/data")
	if err != nil {
		log.Fatal(err)
	}
	g.loop()
}
