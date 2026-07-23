# Developing

## Getting started

In order to test and develop in this repo you will need the following dependencies installed:
- make

After cloning the repo run `make bootstrap` to download go mod dependencies, create the `/.tmp` dir, and download helper utilities.

The main `make` tasks for common static analysis and testing are `lint`, `lint-fix`, and `unit`.

See `make help` for all the current make tasks.
