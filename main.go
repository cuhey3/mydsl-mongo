package main

import (
	"fmt"
	"github.com/cuhey3/mydsl/go"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
)

func main() {
	f, err := os.Open("yamls/router.yml")
	if err != nil {
		fmt.Println("open error:", err)
	}
	defer f.Close()
	yamlInput, err := ioutil.ReadAll(f)
	if err != nil {
		fmt.Println("read error:", err)
	}
	var objInput map[interface{}]interface{}
	yamlError := yaml.UnmarshalStrict(yamlInput, &objInput)
	if yamlError != nil {
		fmt.Println("unmarshal error:", err)
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = ":8080"
	} else {
		port = ":" + port
	}
	container := map[string]interface{}{"PORT": port}
	evaluated, err := mydsl.NewArgument(objInput["main"]).Evaluate(container)
	fmt.Println("container", container)
	fmt.Println("evaluated", evaluated)
	fmt.Println("error", err)
}
