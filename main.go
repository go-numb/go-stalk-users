package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/BurntSushi/toml"
	"github.com/ChimeraCoder/anaconda"
	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

var f *os.File

func init() {
	f, _ = os.OpenFile("./error.log", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	logrus.SetOutput(f)
	logrus.SetLevel(logrus.ErrorLevel)
	logrus.SetFormatter(&logrus.JSONFormatter{})
}

type Client struct {
	config  map[string]interface{}
	tw      *anaconda.TwitterApi
	discord *discordgo.Session
	Targets []string
	Unique  map[string]bool
}

func New() *Client {
	var s map[string]interface{}
	toml.DecodeFile("./config.toml", &s)
	d, err := discordgo.New()
	if err != nil {
		logrus.Fatal(err)
	}

	return &Client{
		config: s,
		tw: anaconda.NewTwitterApiWithCredentials(
			fmt.Sprintf("%v", s["tw_access_token"]),
			fmt.Sprintf("%v", s["tw_access_token_secret"]),
			fmt.Sprintf("%v", s["tw_consumer_key"]),
			fmt.Sprintf("%v", s["tw_consumer_secret"]),
		),
		discord: d,
		Unique:  make(map[string]bool),
	}
}

func (p *Client) Close() error {
	p.tw.Close()
	if err := p.discord.Close(); err != nil {
		return err
	}
	return nil
}

func main() {
	client := New()
	defer client.Close()

	client.Targets = getTargets()
	fmt.Printf("%+v 計%d名\n", client.Targets, len(client.Targets))

	done := make(chan os.Signal)

	var eg errgroup.Group
	ctx, cancel := context.WithCancel(context.Background())

	eg.Go(func() error {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		defer cancel()

		for {
			select {
			case <-ticker.C:
				if err := client.GetAll(); err != nil {
					logrus.Error(err)
				}

			case <-done:
				return fmt.Errorf("get signal")

			case <-ctx.Done():
				return ctx.Err()

			}
		}
	})

	if err := eg.Wait(); err != nil {
		logrus.Fatal(err)
	}

}

func getTargets() []string {
	var targets string
	flag.StringVar(&targets, "t", "", "<-t> set target user_id, can use spaces")
	flag.Parse()

	return strings.Fields(targets)
}

func (p *Client) GetAll() error {
	for i := range p.Targets {
		u := url.Values{}
		u.Add("count", "1")
		u.Add("screen_name", p.Targets[i])
		u.Add("include_entities", "true")
		u.Add("exclude_replies", "false")
		u.Add("include_rts", "true")
		u.Add("tweet_mode", "extended")
		tweets, err := p.tw.GetUserTimeline(u)
		if err != nil {
			return err
		}
		if len(tweets) < 1 {
			continue
		}

		if 0 < len(tweets) {
			// 被りがあれば離脱
			key := getkey(p.Targets[i], tweets[0].IdStr)
			_, ok := p.Unique[key]
			if ok {
				continue
			}

			files := make([]*discordgo.MessageEmbed, len(tweets[0].Entities.Media))
			if len(tweets[0].Entities.Media) != 0 {
				for j := range tweets[0].Entities.Media {
					files[j] = &discordgo.MessageEmbed{
						URL:   tweets[0].Entities.Media[j].Url,
						Title: tweets[0].Entities.Media[j].ExtAltText,
						Type:  tweets[0].Entities.Media[j].Type,
						Image: &discordgo.MessageEmbedImage{
							URL: tweets[0].Entities.Media[j].Media_url,
						},
					}
				}
			}

			_, err := p.discord.WebhookExecute(
				fmt.Sprintf("%v", p.config["discord_webhook_channelid"]),
				fmt.Sprintf("%v", p.config["discord_webhook_token"]),
				true,
				&discordgo.WebhookParams{
					Username:  p.Targets[i],
					AvatarURL: tweets[0].User.ProfileImageURL,
					Content:   fmt.Sprintf("%s のつぶやき\n%s\n%s", tweets[0].User.Name, tweets[0].FullText, tweets[0].CreatedAt),
					Embeds:    files,
				},
			)
			if err != nil {
				logrus.Error(err)
				continue
			}

			// 被りなしの最新ツイートがあれば、過去のものを削除してメモリ解放
			for k := range p.Unique {
				if !strings.HasPrefix(k, p.Targets[i]) {
					continue
				}
				delete(p.Unique, k)
			}
			p.Unique[key] = true
		}

		time.Sleep(5 * time.Second)
	}

	return nil

}

func getkey(id, tID string) string {
	return fmt.Sprintf("%s:%s", id, tID)
}
