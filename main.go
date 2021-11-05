package main

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	fmt.Println("Connected")
}

var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	fmt.Printf("Connect lost: %v", err)
}

var messagePubHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	fmt.Printf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())
}

var telldusString = "16:TDRawDeviceEvent95:class:command;protocol:arctech;model:selflearning;house:29145578;unit:2;group:0;method:turnoff;i1s"

func main() {
	fmt.Println("Starting!")

	var broker = "mqtt.example.com"
	var port = 1883
	var username = "telldus"
	var password = "secret"
	var eventSocket = "/tmp/TelldusEvents"

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%d", broker, port))
	opts.SetClientID("telldusbridge")
	opts.SetUsername(username)
	opts.SetPassword(password)
	opts.SetDefaultPublishHandler(messagePubHandler)
	opts.OnConnect = connectHandler
	opts.OnConnectionLost = connectLostHandler
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	c, err := net.Dial("unix", eventSocket)
	if err != nil {
		panic(err.Error())
	}

	for {
		// Read line
		buf := make([]byte, 512)
		nr, err := c.Read(buf)
		if err != nil {
			return
		}

		data := buf[0:nr]
		fmt.Printf("Received: %v\n", string(data))

		_, paramstrings, _ := splitTelldus(string(data))

		params := getParams(paramstrings)

		if params["protocol"] == "arctech" {
			singleString := params["house"] + "-" + params["unit"] + "-" + params["group"] + "-" + params["method"]
			send(client, singleString)
		}

		jsonString, err := json.Marshal(params)
		if err != nil {
			panic(err)
		}

		// Send message
		send(client, string(jsonString))

	}
}

func send(client mqtt.Client, message string) {
	// Send message

	fmt.Printf("Sending '%v'\n", message)
	token := client.Publish("telldus/event", 0, false, message)
	token.Wait()
}

func sub(client mqtt.Client) {
	topic := "#"
	token := client.Subscribe(topic, 1, nil)
	token.Wait()
	fmt.Printf("Subscribed to topic: %s", topic)
}

func splitTelldus(message string) (string, []string, string) {
	fields := strings.Split(message, ";")

	var header string
	var paramstrings []string
	var rest string

	header = fields[0]
	if len(fields) > 1 {
		paramstrings = fields[1:len(fields)-1]
	}

	if len(fields) > 1 {
		rest = fields[len(fields)-1]
	}

	return header, paramstrings, rest
}

func getParams(paramFields []string) map[string]string {
	params := make(map[string]string)

	for _, i := range paramFields {
		kv := strings.Split(i, ":")
		key, val := kv[0], kv[1]

		params[key] = val
	}

	return params
}
