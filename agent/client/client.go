package client

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/kkEo/g-mk8s/agent/model"
)

type Client struct {
	Url    string
	ApiKey string
}

func (c *Client) Next() *model.Blueprint {

	req, err := http.NewRequest("GET", c.Url, nil)
	req.Header.Set("Authorization", c.ApiKey)

	if err != nil {
		log.Fatalf("Error creating request: %s", err)
		return nil
	}

	client := &http.Client{}

	resp, err := client.Do(req)

	if err != nil {
		log.Fatalf("Error sending request: %s", err)
		return nil
	}

	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return nil
	}

	log.Printf("Resp: %s", string(bodyBytes))

	var obj model.Blueprint
	err = json.Unmarshal(bodyBytes, &obj)
	if err != nil {
		fmt.Printf("Error parsing JSON data: ", err)
	}

	return &obj
}
