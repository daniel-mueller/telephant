package main

import (
	"fmt"
	"log"
	"os"

	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/qml"
	"github.com/therecipe/qt/quickcontrols2"

	"github.com/muesli/chirp/accounts/mastodon"
)

var (
	config               Config
	conversationModel    = NewMessageModel(nil)
	accountMessagesModel = NewMessageModel(nil)
)

func connectToInstance(instance string) {
	var authURI string
	var redirectURI string
	var err error
	tc, authURI, redirectURI, err = mastodon.RegisterAccount(instance)

	fmt.Println("auth uri:", authURI)
	fmt.Println("redirect uri:", redirectURI)
	fmt.Println(err)
}

func authInstance(code string) {
	instance, token, clientID, clientSecret, err := tc.Authenticate(code)
	fmt.Println("authenticate:", err)
	if err != nil {
		return
	}

	config.Account[0].Instance = instance
	config.Account[0].ClientID = clientID
	config.Account[0].ClientSecret = clientSecret
	config.Account[0].Token = token
	setupMastodon(config.Account[0])
}

// reply is used to post a new message
// if replyid is > 0, it's send as a reply
func reply(replyid string, message string) {
	var err error
	if replyid != "" {
		log.Println("Sending reply to:", replyid, message)
		err = tc.Reply(replyid, message)
	} else {
		log.Println("Posting:", message)
		err = tc.Post(message)
	}
	if err != nil {
		log.Println("Error posting to Account:", err)
	}
}

// share a post
func share(id string) {
	log.Println("Sharing:", id)
	if err := tc.Share(id); err != nil {
		log.Println("Error posting to Account:", err)
	}
}

// unshare a post
func unshare(id string) {
	log.Println("Unsharing:", id)
	if err := tc.Unshare(id); err != nil {
		log.Println("Error posting to Account:", err)
	}
}

// like a post
func like(id string) {
	log.Println("Liking:", id)
	if err := tc.Like(id); err != nil {
		log.Println("Error posting to Account:", err)
	}
}

// unlike a post
func unlike(id string) {
	log.Println("Unliking:", id)
	if err := tc.Unlike(id); err != nil {
		log.Println("Error posting to Account:", err)
	}
}

// follow changes the relationship to another user
func follow(id string, follow bool) {
	if follow {
		log.Println("Following:", id)
		if err := tc.Follow(id); err != nil {
			log.Println("Error following user:", err)
			return
		}
	} else {
		log.Println("Unfollowing:", id)
		if err := tc.Unfollow(id); err != nil {
			log.Println("Error unfollowing user:", err)
			return
		}
	}

	profileBridge.SetFollowing(follow)
}

// loadConversation loads a message thread
func loadConversation(id string) {
	log.Println("Loading conversation:", id)
	messages, err := tc.LoadConversation(id)
	if err != nil {
		log.Println("Error loading conversation:", err)
		return
	}

	fmt.Println("Found conversation posts:", len(messages))
	conversationModel.Clear()
	for _, m := range messages {
		p := messageFromEvent(m)
		conversationModel.AppendMessage(p)
	}
}

// loadAccount loads an entire profile
func loadAccount(id string) {
	log.Println("Loading account:", id)
	profile, messages, err := tc.LoadAccount(id)
	if err != nil {
		log.Println("Error loading account:", err)
		return
	}

	profileBridge.SetUsername(profile.Username)
	profileBridge.SetName(profile.Name)
	profileBridge.SetAvatar(profile.Avatar)
	profileBridge.SetProfileURL(profile.ProfileURL)
	profileBridge.SetProfileID(profile.ProfileID)
	profileBridge.SetPosts(profile.Posts)
	profileBridge.SetFollowCount(profile.FollowCount)
	profileBridge.SetFollowerCount(profile.FollowerCount)
	profileBridge.SetFollowing(profile.Following)
	profileBridge.SetFollowedBy(profile.FollowedBy)

	fmt.Println("Found account posts:", len(messages))
	accountMessagesModel.Clear()
	for _, m := range messages {
		p := messageFromEvent(m)
		accountMessagesModel.AppendMessage(p)
	}
}

// runApp loads and executes the QML UI
func runApp(config Config) {
	quickcontrols2.QQuickStyle_SetStyle(config.Style)

	app := qml.NewQQmlApplicationEngine(nil)
	app.RootContext().SetContextProperty("uiBridge", uiBridge)
	app.RootContext().SetContextProperty("accountBridge", accountBridge)
	app.RootContext().SetContextProperty("profileBridge", profileBridge)
	app.RootContext().SetContextProperty("settings", configBridge)

	app.Load(core.NewQUrl3("qrc:/qml/chirp.qml", 0))
	gui.QGuiApplication_Exec()
}

// setupMastodon starts a new Mastodon client and sets up event handling & models for it
func setupMastodon(config Account) {
	tc = mastodon.NewAccount(config.Instance, config.Token, config.ClientID, config.ClientSecret)
	postModel := NewMessageModel(nil)
	notificationModel := NewMessageModel(nil)

	accountBridge.SetUsername("Logging in...")
	accountBridge.SetMessages(postModel)
	accountBridge.SetNotifications(notificationModel)
	accountBridge.SetConversation(conversationModel)
	accountBridge.SetAccountMessages(accountMessagesModel)

	evchan := make(chan interface{})
	go handleEvents(evchan, postModel, notificationModel)
	go func() {
		tc.Run(evchan)
	}()
}

func main() {
	core.QCoreApplication_SetApplicationName("Chirp")
	core.QCoreApplication_SetOrganizationName("fribbledom.com")
	core.QCoreApplication_SetAttribute(core.Qt__AA_EnableHighDpiScaling, true)
	gui.NewQGuiApplication(len(os.Args), os.Args)

	setupQmlBridges()

	// load config
	config = LoadConfig()
	if config.Style == "" {
		config.Style = "Material"
	}
	configBridge.SetStyle(config.Style)

	setupMastodon(config.Account[0])
	runApp(config)

	// save config
	config.Style = configBridge.Style()
	SaveConfig(config)
}
