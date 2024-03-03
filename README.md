# Goft

Goft is a chat rooms application written to prioritize simplicity by utilizing
the htmx library and to explores the htmx potentials to replace the bloated
javascript libraries.

# Running this project

Prerequisite: 

1. You must be having postgres installed and running
2. nodejs is used for prettier formatting and tailwindcss

Follow steps:

1. install the mage:
```sh
go install github.com/magefile/mage@v1.15.0
```

2. initialize the project  
```sh
mage init
```

which is equivalent to following command

```sh
mage install # install dependencies
mage migrate # database migrations 
mage seed # database seeding
```

3. use `.env.example` as a example for your own `.env`

4. and finally run

```sh
mage run
```

to see all mage target run
```sh
mage -l
```

# Todos
- [ ] Docker
- [ ] show user name and date of every message
