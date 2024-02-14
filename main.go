package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

var lastReleaseTitle string

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(".envファイルの読み取りに失敗しました。")
	}

	discordToken := os.Getenv("DISCORD_BOT_TOKEN")
	channelID := os.Getenv("CHANNEL_ID")
	repoOwner := "owner"
	repoName := "repo"

	dg, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		fmt.Println("ディスコードとのセッションが確立できません", err)
		return
	}

	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		messageCreate(s, m, channelID, repoOwner, repoName)
	})

	err = dg.Open()
	if err != nil {
		fmt.Println("ディスコードとのセッションがオープンできません", err)
		return
	}

	fmt.Println("BOTが実行されました。")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	dg.Close()
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate, channelID, repoOwner, repoName string) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	if m.Content == "!checkreleases" {
		releases, err := checkGitHubReleases(repoOwner, repoName)
		if err != nil {
			log.Printf("リリースのチェックに失敗: %v", err)
			return
		}

		if len(releases) > 0 {
			latestRelease := releases[0]
			if latestRelease.Title != lastReleaseTitle {
				s.ChannelMessageSend(m.ChannelID, "新しいリリースが見つかりました。: "+latestRelease.
					Title+" ("+
					""+latestRelease.Link.Href+")")
				lastReleaseTitle = latestRelease.Title
			} else {
				s.ChannelMessageSend(m.ChannelID, "新しいリリースはありません。")
			}
		}
	}
}

func checkGitHubReleases(owner, repo string) ([]Release, error) {
	rssURL := fmt.Sprintf("https://github.com/%s/%s/releases.atom", owner, repo)
	fmt.Println("RSS URL:", rssURL)

	resp, err := http.Get(rssURL)
	if err != nil {
		log.Printf("HTTPSリクエストエラー: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("HTTPステータスコードエラー: %d", resp.StatusCode)
		return nil, fmt.Errorf("RSSの取得に失敗しました。 %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("HTTPレスポンスボディ読み取りエラー: %v", err)
		return nil, err
	}

	log.Printf("取得したRSSデータ: %s", body)

	var feed AtomFeed
	err = xml.Unmarshal(body, &feed)
	if err != nil {
		log.Printf("XMLアンマーシャリングエラー: %v", err)
		return nil, err
	}

	var releases []Release
	for _, entry := range feed.Entries {
		releases = append(releases, Release{
			Title: entry.Title,
			Link:  entry.Link,
		})
	}

	log.Printf("取得したリリース: %v", releases)

	return releases, nil
}

type Release struct {
	Title string   `xml:"title"`
	Link  AtomLink `xml:"link"`
}

type AtomFeed struct {
	XMLName xml.Name  `xml:"feed"`
	Entries []Release `xml:"entry"`
}

type AtomLink struct {
	Href string `xml:"href,attr"`
}
