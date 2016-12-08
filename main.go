package main

import (
	//"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/eclipse/paho.mqtt.golang"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	//"sync"
	//"regexp"
	"strings"
	"syscall"
	"time"
)

type Playlist struct {
	ItemList []Item `xml:"item"`
}

type Item struct {
	Title        string `xml:"title"`
	Performer    string `xml:"performer"`
	Source       string `xml:"source"`
	Duration     int    `xml:"duration"`
	HQ           string `xml:"hq"`
	Link         string `xml:"link"`
	ErrorMessage string `xml:"errormessage"`
	ErrorCode    int    `xml:"errorcode"`
}

type Result struct {
	Item   Item
	Source string
}

type ControlMessage struct {
	Url           string
	ResponseTopic string
}

var (
	TOPIC      = "music-downloader/control"
	QOS   byte = 1
)

func connectToBroker(client mqtt.Client) {
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Println("Fail to connect!")
		log.Printf("%v\n", token.Error())
	} else {
		log.Println("Connected to broker!")
		if token := client.Subscribe(TOPIC, QOS, func(c mqtt.Client, m mqtt.Message) {
			//msg := fmt.Sprintf("%s", m.Payload())
			handleControlMessage(client, m.Payload())
			//postKodiMessage(msg)
		}); token.Wait() && token.Error() != nil {
			log.Println("Fail to connect!")
			log.Printf("%v\n", token.Error())
		} else {
			log.Println("Subscribed to command topic!")
		}
	}
}

func handleControlMessage(client mqtt.Client, payload []byte) {
	var ctlMsg ControlMessage
	err := json.Unmarshal(payload, &ctlMsg)
	if err != nil {
		log.Printf("Failed to parse control message %s\n", payload)
		return
	}

	playlistUrl, err := url.Parse(ctlMsg.Url)
	if err != nil {
		log.Printf("Failed to parse url %s\n", ctlMsg.Url)
		return
	}

	log.Printf("Processing playlist %s\n", playlistUrl)

	playlist, err := processPlaylistUrl(ctlMsg.Url)
	if err != nil {
		log.Printf("Fail to process playlist %v\n", err)
		return
	} else {
		log.Println("Process playlist done!")
	}
	data, err := json.Marshal(playlist)
	log.Printf("Publishing response message to %s\n", ctlMsg.ResponseTopic)
	token := client.Publish(ctlMsg.ResponseTopic, 1, false, data)
	if token.Wait(); token.Error() != nil {
		log.Printf("Failed to publish response data to %s\n, Error: %v\n", ctlMsg.ResponseTopic, err)
	}
}

func processPlaylistUrl(playlistUrl string) (playlist Playlist, err error) {
	resp, err := http.Get(playlistUrl)
	if err != nil {
		return playlist, err
	}
	defer resp.Body.Close()
	//var playlist Playlist
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return playlist, err
	}
	err = xml.Unmarshal(body, &playlist)
	if err != nil {
		return playlist, err
	}

	//var wg sync.WaitGroup
	//results := make(chan Result)
	results := make(chan int)
	for _, item := range playlist.ItemList {
		go func(item *Item) error {
			//wg.Add(1)
			resp, err := http.Get(item.Source)
			if err != nil {
				log.Printf("Error processing %s\nhttp.Get => %v\n", item.Title, err.Error())
			}
			// Overrid original url
			//item.Source = strings.Replace(resp.Request.URL.String(), "&", "&amp;", -1)
			//item.Source = "<![CDATA[" + strings.Replace(resp.Request.URL.String(), "&", "&amp;", -1) + "]]"
			item.Source = resp.Request.URL.String()
			results <- 0
			//wg.Done()

			return nil
		}(&item)
	}
	//time.Sleep(1 * time.Second)
	//wg.Wait()

	for i := 0; i < len(playlist.ItemList); i++ {
		<-results
		//result := <-results
		//result.Item.Source = result.Source
	}

	return playlist, nil
}

func escapeString(input string) string {
	return strings.Trim(strings.Replace(input, "\n", "", -1), " ")
}

func main() {
	f, err := os.OpenFile("mqtt.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	log.SetOutput(f)
	opt := mqtt.NewClientOptions()
	opt.AddBroker("tcp://iot.eclipse.org:1883")
	clientId := fmt.Sprintf("music-downloader-agent-%d", time.Now().Nanosecond())
	log.Printf("Using client id: %s\n", clientId)
	opt.SetClientID(clientId)

	client := mqtt.NewClient(opt)

	connectToBroker(client)

	go func(__client mqtt.Client) {
		for {
			if !__client.IsConnected() {
				log.Printf("Connection to broker is lost. Retrying...\n")
				connectToBroker(__client)
			}
			time.Sleep(10 * time.Second)
		}
	}(client)

	defer client.Disconnect(500)
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL, os.Interrupt, os.Kill)
	for {
		sig, success := <-sigchan
		if !success ||
			sig == syscall.SIGINT ||
			sig == syscall.SIGTERM ||
			sig == syscall.SIGKILL ||
			sig == os.Interrupt ||
			sig == os.Kill {
			break
		}
	}
}
