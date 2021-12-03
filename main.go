package main

import (
	"os"
	"encoding/json"
	"fmt"
	"net"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	yaml "gopkg.in/yaml.v2"
	defaults "github.com/creasty/defaults"
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

func main() {
	fmt.Println("Starting!")

	fmt.Println("Reading config file")
	var systemConfigFile = "/etc/telldusmqtt.conf"
	var config Config

	ReadConfig(systemConfigFile, &config)

	var broker = config.MqttServer.BrokerHost
	var port = config.MqttServer.BrokerPort
	fmt.Printf("Config host: %s:%s\n", broker, port)
	var username = config.MqttServer.Username
	var password = config.MqttServer.Password
	var clientId = config.MqttServer.ClientId
	var eventSocket = config.TelldusBridge.TelldusEventsSocket

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%s", broker, port))
	opts.SetClientID(clientId)
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

type Config struct {
  MqttServer struct {
    BrokerHost string `yaml:"brokerhost"`
    BrokerPort string `yaml:"brokerport" default:"1883"`
    Username string `yaml:"username"`
    Password string `yaml:"password"`
    ClientId string `yaml:"clientid" default:"telldusbridge"`
  } `yaml:"mqtt"`

  TelldusBridge struct {
    TelldusEventsSocket string `yaml:"eventsocket" default:"/tmp/TelldusEvents"`
    MqttEventTopic string `yaml:"mqtttopic" "telldus/event"`
  } `yaml:"events"`

}

func ReadConfig(filename string, config *Config) {

	defaults.Set(config)

  f, err := os.Open(filename)
  if err != nil {
    printError(err)
  }
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(&config)
	if err != nil {
		printError(err)
	}

	fmt.Println(config)

}

func printError(err error) {
	fmt.Println(err)
	os.Exit(2)
}
