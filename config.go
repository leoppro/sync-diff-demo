package main

type Config struct {
	Task Task `toml:"task"`
}

type Task struct {
	OutputDir string `toml:"output-dir"`
}
