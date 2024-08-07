package handlers

import (
	"messages/app/db"
	"messages/app/helpers"
	"messages/app/models"
	"time"

	"github.com/anthdm/superkit/kit"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

type Message struct {
	Title   string `json:"title"`
	Message string `json:"message"`
	Type    string `json:"type"`
}

type Response struct {
	Origin   string    `json:"domain"`
	Messages []Message `json:"messages"`
	Error    string    `json:"error,omitempty"`
}

func HandleApi(kit *kit.Kit) error {
	loc, _ := time.LoadLocation(kit.Getenv("TIMEZONE", "America/Toronto"))

	request := kit.Request
	kit.Response.Header().Set("Content-Type", "application/json")

	lang := request.Header.Get("Accept-Language")
	if lang == "" {
		lang = "en"
	}

	originDomain := request.Header.Get("Origin")
	response := Response{
		Origin:   originDomain,
		Messages: make([]Message, 0),
	}

	timezone := request.Header.Get("Timezone")
	if timezone != "" {
		var err error
		loc, err = time.LoadLocation(timezone)
		if err != nil {
			response.Error = "Invalid timezone"
			kit.JSON(400, response)
			return nil
		}
	}

	if !helpers.IsValidDomain(originDomain) {
		response.Error = "Invalid domain"
		kit.JSON(400, response)
		return nil
	}

	if !IsValidLanguage(lang) {
		response.Error = "Invalid language"
		kit.JSON(400, response)
		return nil
	}

	dbWebsite, err := models.Websites(
		models.WebsiteWhere.URL.EQ(originDomain),
	).One(kit.Request.Context(), db.Query)
	if err != nil {
		response.Error = "Unknown domain"
		kit.JSON(400, response)
		return nil
	}

	messagesIds, err := models.WebsitesMessages(
		models.WebsitesMessageWhere.WebsiteId.EQ(dbWebsite.ID),
	).All(kit.Request.Context(), db.Query)
	if err != nil {
		kit.JSON(200, response)
		return nil
	}

	messagesIdsList := make([]int64, 0, len(messagesIds))
	for _, message := range messagesIds {
		messagesIdsList = append(messagesIdsList, message.MessageId)
	}

	mod := []qm.QueryMod{
		models.MessageWhere.ID.IN(messagesIdsList),
		models.MessageWhere.Language.EQ(lang),
		models.MessageWhere.DisplayTo.GT(time.Now().In(loc)),
	}

	if !dbWebsite.Staging {
		mod = append(mod, models.MessageWhere.DisplayFrom.LT(time.Now().In(loc)))
	}

	dbMessageList, err := models.Messages(mod...).All(kit.Request.Context(), db.Query)

	response.Messages = make([]Message, 0, len(dbMessageList))
	for _, dbMessage := range dbMessageList {
		message := Message{
			Title:   dbMessage.Title,
			Message: string(mdToHTML([]byte(dbMessage.Message))),
			Type:    dbMessage.Type,
		}
		if dbWebsite.Staging && dbMessage.DisplayFrom.After(time.Now().In(loc)) {
			message.Message = "[Preview] " + message.Message
		}

		response.Messages = append(response.Messages, message)
	}
	kit.JSON(200, response)
	return nil
}

func IsValidLanguage(lang string) bool {
	return lang == "en" || lang == "fr"
}

func mdToHTML(md []byte) []byte {
	// create markdown parser with extensions
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse(md)

	// create HTML renderer with extensions
	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)

	return markdown.Render(doc, renderer)
}
