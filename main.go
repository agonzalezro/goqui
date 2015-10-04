package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jmcvetta/neoism"
)

var (
	errCanNotMove              = errors.New("Can not move to the given direction")
	errNoSuchPickableObject    = errors.New("No such pickable object")
	errItemNotUsableWithObject = errors.New("The item can not be used with that object")
	errPersonNotFound          = errors.New("The person does not exist")
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
		  MATCH(:PLAYER)-[:IS_IN]-(:ROOM)-[r:MOVE|MOVE_WITH_CONDITION]->(:ROOM)
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
		  MATCH (:PLAYER)-[:IS_IN]-(:ROOM)-[:CONTENTS|SEE]-(o)
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
		return g.moveWithCondition(d)
	}
	return nil
}

func (g Game) moveWithCondition(d string) error {
	exits := []struct {
		Count int `json:"count(r)"`
	}{}

	cq := neoism.CypherQuery{
		Statement: `
		  MATCH (p:PLAYER)-[i:IS_IN]-(:ROOM)-[m:MOVE_WITH_CONDITION]->(r:ROOM),
		        ()-[u:USABLE_WITH]->(object:OBJECT)<-[:USED_WITH]-()
		  WHERE m.direction = "north" AND u.condition = m.condition
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
		return errCanNotMove
	}
	return nil
}

func (g Game) pick(o string) error {
	exists := []struct {
		Count int `json:"count(o)"`
	}{}

	cq := neoism.CypherQuery{
		Statement: `
		  MATCH (p:PLAYER)-[:IS_IN]-(:ROOM)-[c:CONTENTS]-(o:OBJECT{pickable:true})
		  WHERE o.name = {o}
		  DELETE c
		  CREATE(p)-[:OWNS]->(o)
		  RETURN count(o)
		`,
		Parameters: neoism.Props{"o": o},
		Result:     &exists,
	}

	if err := g.db.Cypher(&cq); err != nil {
		// TODO
	}

	if exists[0].Count == 0 {
		return errNoSuchPickableObject
	}
	return nil
}

func (g Game) use(item, object string) (string, error) {
	result := []struct {
		Condition string `json:"r.condition"`
	}{}

	cq := neoism.CypherQuery{
		Statement: `
	      MATCH (:PLAYER)-[:OWNS]-(item:OBJECT)-[r:USABLE_WITH]-(object:OBJECT)
		  WHERE item.name = {item} AND object.name = {object}
		  CREATE (item)-[:USED_WITH]->(object)
		  RETURN r.condition
		`,
		Parameters: neoism.Props{"item": item, "object": object},
		Result:     &result,
	}

	if err := g.db.Cypher(&cq); err != nil {
		// TODO
	}

	if len(result) == 0 {
		return "", errItemNotUsableWithObject
	}
	return result[0].Condition, nil
}

func (g Game) talk(person string) (string, error) {
	result := []struct {
		Message string `json:"p.message"`
	}{}

	cq := neoism.CypherQuery{
		Statement: `
				MATCH (:PLAYER)-[:IS_IN]-(:ROOM)-[:SEE]-(p:PERSON)
				WHERE p.name =~ {person}
				RETURN p.message
				`,
		Parameters: neoism.Props{"person": "(?i)" + person},
		Result:     &result,
	}

	if err := g.db.Cypher(&cq); err != nil {
		// TODO
		log.Fatal(err)
	}

	if len(result) == 0 {
		return "", errPersonNotFound
	}
	return result[0].Message, nil
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

func cls() {
	// TODO: this should be multiplaform
	print("\033[H\033[2J")
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

		var phraseDirectObject []string
		if len(action) > 1 {
			phraseDirectObject = action[1:] // expand this
		}

		switch action[0] {
		case "move":
			if err := g.move(phraseDirectObject[0]); err != nil {
				fmt.Println("I can't move there")
				fmt.Println("Tip: use DESCRIBE to see your possible exits.")
				break
			}
		case "pick": // TODO: get as well
			object := phraseDirectObject[0]
			if err := g.pick(object); err != nil {
				fmt.Println("You can't pick that")
				fmt.Println("Tip: use DESCRIBE to see the available objects, write: PICK object to get it.")
				break
			}
			fmt.Printf("You picked %s\n", object)
		case "use":
			item := phraseDirectObject[0]
			object := phraseDirectObject[2] // TODO: 1 is "with", OMG huge assumption here
			condition, err := g.use(item, object)
			if err != nil {
				fmt.Println("You can't do that.")
				fmt.Printf("Tip: Do you own '%s'?\n", item) // TODO: return several errors to know how to "tip"
				break
			}
			fmt.Printf("The %s is now %s\n", object, strings.Split(condition, ".")[1])
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
		case "talk":
			person := phraseDirectObject[1] // TODO: assuming "to" is the index 0
			message, err := g.talk(person)
			if err != nil {
				fmt.Printf("You can't talk to: '%s'!\n", person)
				break
			}
			fmt.Println(message)
		default:
			fmt.Println("I can't do that!")
			fmt.Println("Tip: possible actionss are: MOVE, PICK, USE, INVENTORY & DESCRIBE")
		}

		fmt.Printf("\n%s\n\n", strings.Repeat("-", 80))
	}
}

func main() {
	var neo4j = flag.String("neo4j", "http://neo4j:password@b2d:7474/db/data", "the neo4j URI")
	flag.Parse()

	g, err := NewGame(*neo4j)
	if err != nil {
		log.Fatal(err)
	}
	g.loop()
}
