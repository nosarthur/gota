# gota: manage multiple git repos with sanity

This is a remake of the [gita](https://github.com/nosarthur/gita) project with golang.
The motivations are

- speed improvements
    - eliminate the slow Python startup time
    - call go-git API instead of subprocess git commands
- ease of maintenance
    - Separate model from view
    - The Python asyncio APIs keep changing


## Installation

1. clone the repo
2. `make`
3. run `gota` command


## Model
