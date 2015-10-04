goqui{,xote}
============

`goqui` (aka `goquixote`) is a small experiment I made for fun. Basically it's
an engine to use Neo4J as the backend of your old school text adventure games.

In the folder `examples/` you will find some games (just one for now TBH) that
you can import to your Neo4J to test this code.

How does it look like?

`````
This is a demo of goqui, I think that for now you should just move north :)
You see: nothing.
Your possible moves?:
- north

>
`````

But the most important bit, how does it look in Neo4J?

![neoscreen](examples/demo_adventure.png)

**Use this under your own responsability, I just created it for fun and the
code is ugly and untested. Probably you are not going to break anything
serious, but I wouldn't run this using my production DBs. Also if you want
to learn some Go or Neo4J you should pick some other project out there :)**

Quick start
-----------

1. Run Neo4J with Docker's help:

    docker run -i -t -d --name neo4j --cap-add=SYS_RESOURCE -p 7474:7474 agonzalezro/neo4j

2. Go to your docker host, port `7474` and import one of the examples.
3. If you want to see how it looks:

    MATCH (n)
    OPTIONAL MATCH (n)-[r]-()
    RETURN n, r

4. Get `goqui` if you didn't yet:

    go get github.com/agonzalezro/goqui

4. Run the program against your Neo4J:

    goqui -neo4j http://neo4j:password@b2d:7474/db/data"

Playing
-------

Sadly, the CLI is very dumb for now. The action that you can perform are:

- **move** *direction*
- **pick** *object*
- **use** *object* **with** object
- **inventory**
- **talk**
- **describe**

**Please, remember that you need to write those commands as they are, don't
forget the articles or prepositions!**

Developing
----------

I am using the Go 1.5 vendor experiment, so remember to set
`GO15VENDOREXPERIMENT` to `1`.

TODO
----

- Explain how to create new adventures.
- Lost of duplications everywhere.
- Implement a Go-interface to easily change the game-interface.
- Provide binaries.
- The CLI should be hundred time more clever.
- All the queries should be case insensitive
- Return the message of `MOVE_WITH_CONDITION` when you can not move, for
  example: "you need to open the door before moving".
- Tests? Yeah, this code was made just for fun but it should have some.
- End the game if it's the final room.
