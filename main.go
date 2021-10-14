package main

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

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

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%d", broker, port))
	opts.SetClientID("telldusbridge")
	opts.SetUsername("telldus")
	opts.SetPassword("secret")
	opts.SetDefaultPublishHandler(messagePubHandler)
	opts.OnConnect = connectHandler
	opts.OnConnectionLost = connectLostHandler
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	c, err := net.Dial("unix", "/tmp/TelldusEvents")
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
		fmt.Printf("Received: %v", string(data))

		_, paramstrings, _ := splitTelldus(string(data))

		params := getParams(paramstrings)

		jsonString, err := json.Marshal(params)
		if err != nil {
			panic(err)
		}

		fmt.Println(string(jsonString))

		// Send messge
		token := client.Publish("telldus/event", 0, false, string(jsonString))
		token.Wait()
		time.Sleep(time.Second)

	}
}
func sub(client mqtt.Client) {
	topic := "#"
	token := client.Subscribe(topic, 1, nil)
	token.Wait()
	fmt.Printf("Subscribed to topic: %s", topic)
}

func splitTelldus(message string) (string, []string, string) {
	fields := strings.Split(telldusString, ";")

	header, paramstrings, rest :=
		fields[0],
		fields[1:len(fields)-1],
		fields[len(fields)-1]

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
