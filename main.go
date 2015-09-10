package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jmcvetta/neoism"
)

var (
	CanNotMoveErr        error = errors.New("Can not move to the given direction")
	NoSuchPickableObject error = errors.New("No such pickable object")
)

type Game struct {
	db *neoism.Database
}

type content struct {
	Name        string `json:"o.name"`
	Description string `json:"o.description"`
}

type room struct {
	description string
	exits       []string
	contents    []content
}

func NewGame(uri string) (game *Game, err error) {
	g := Game{}

	g.db, err = neoism.Connect(uri)
	if err != nil {
		return nil, err
	}

	return &g, nil
}

func NewRoom(d <-chan string, cs <-chan content, es <-chan string) *room {
	var contents []content
	var exits []string
	description := <-d

	for c := range cs {
		contents = append(contents, c)
	}

	for e := range es {
		exits = append(exits, e)
	}

	return &room{
		description: description,
		contents:    contents,
		exits:       exits,
	}
}

func (g Game) getRoomExits(es chan<- string) {
	defer close(es)

	exits := []struct {
		Direction string `json:"r.direction"`
	}{}

	cq := neoism.CypherQuery{
		Statement: `
		  MATCH(:PLAYER)-[:IS_IN]-(:ROOM)-[r:MOVE]->(:ROOM)
		  RETURN r.direction
		`,
		Result: &exits,
	}

	if err := g.db.Cypher(&cq); err != nil {
		// TODO
		panic(err)
	}

	for _, e := range exits {
		es <- e.Direction
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

func (g Game) getRoomContents(cs chan<- content) {
	defer close(cs)

	var contents []content

	cq := neoism.CypherQuery{
		Statement: `
		  MATCH (:PLAYER)-[:IS_IN]-(:ROOM)-[:CONTENTS]-(o:OBJECT)
		  RETURN o.name, o.description
		`,
		Result: &contents,
	}

	if err := g.db.Cypher(&cq); err != nil {
		// TODO
	}

	for _, c := range contents {
		cs <- c
	}
}

func (g Game) move(d string) error {
	exits := []struct {
		Count int `json:"count(r)"`
	}{}

	cq := neoism.CypherQuery{
		Statement: `
          MATCH (p:PLAYER)-[i:IS_IN]-(:ROOM)-[m:MOVE]->(r:ROOM)
		  WHERE m.direction = {d}
		  DELETE i
		  CREATE (p)-[:IS_IN]->(r)
		  RETURN count(r)
		`,
		Parameters: neoism.Props{"d": d},
		Result:     &exits,
	}

	if err := g.db.Cypher(&cq); err != nil {
		// TODO
	}

	if exits[0].Count == 0 {
		return CanNotMoveErr
	}
	return nil
}

func (g Game) pick(o string) error {
	exists := []struct {
		Count int `json:"count(r)"`
	}{}

	cq := neoism.CypherQuery{
		Statement: `
		  MATCH (p:PLAYER)-[:IS_IN]-(:ROOM)-[c:CONTENTS]-(o:OBJECT{pickable:true})
		  WHERE o.name = {o}
		  DELETE c
		  CREATE(p)-[:OWNS]->(o)
		  RETURN count(c)
		`,
		Parameters: neoism.Props{"o": o},
		Result:     &exists,
	}

	if err := g.db.Cypher(&cq); err != nil {
		// TODO
	}

	if exists[0].Count == 0 {
		return NoSuchPickableObject
	}
	return nil
}

func (g Game) currentRoom() *room {
	d := make(chan string)
	go g.getRoomDescription(d)

	cs := make(chan content)
	go g.getRoomContents(cs)

	es := make(chan string)
	go g.getRoomExits(es)

	return NewRoom(d, cs, es)
}

func (g Game) inventory() []content {
	var os []content

	cq := neoism.CypherQuery{
		Statement: `
		  MATCH (:PLAYER)-[:OWNS]-(o:OBJECT)
		  RETURN o.name, o.description
		`,
		Result: &os,
	}

	if err := g.db.Cypher(&cq); err != nil {
		// TODO
	}

	return os
}

func (r room) describe() {
	fmt.Println(r.description)

	fmt.Printf("You see: ")
	if len(r.contents) != 0 {
		for _, c := range r.contents {
			fmt.Printf("\n- %s: %s", c.Name, c.Description)
		}
		fmt.Println()
	} else {
		fmt.Println("nothing.")
	}

	fmt.Printf("Your possible moves?: ")
	if len(r.exits) != 0 {
		for _, e := range r.exits {
			fmt.Printf("\n- %s", e)
		}
		fmt.Println()
	} else {
		fmt.Println("actually, there is not an easy way to go out from here.")
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
		g.currentRoom().describe()

		fmt.Printf("\nWhat do you want to do?: ")
		input, _ := reader.ReadString('\n')
		action := _cleanInput(input)

		directObject := ""
		if len(action) > 1 {
			directObject = action[1] // expand this
			fmt.Println(directObject)
		}

		switch action[0] {
		case "move":
			if err := g.move(directObject); err != nil {
				fmt.Println("I can't move there")
			}
		case "pick":
			if err := g.pick(directObject); err != nil {
				fmt.Println("You can't pick that")
			}
			fmt.Printf("You picked %s\n", directObject)
		case "inventory":
			inventory := g.inventory()
			fmt.Printf("You have: ")
			if len(inventory) != 0 {
				for _, o := range inventory {
					fmt.Printf("\n- %s: %s", o.Name, o.Description)
				}
				fmt.Println()
			} else {
				fmt.Println("nothing, you are quite poor my friend.")
			}
		default:
			fmt.Println("I can't do that!")
		}

		fmt.Printf("\n%s\n\n", strings.Repeat("-", 80))
	}
}

func main() {
	g, err := NewGame("http://neo4j:password@b2d:7474/db/data")
	if err != nil {
		log.Fatal(err)
	}
	g.loop()
}
