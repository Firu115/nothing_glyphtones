package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"glyphtones/database"
	"glyphtones/templates/components"
	"glyphtones/templates/views"
	"glyphtones/utils"
	"log"
	"maps"
	"math/rand/v2"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/labstack/echo/v4"
	"golang.org/x/oauth2"
	godiacritics "gopkg.in/Regis24GmbH/go-diacritics.v2"
)

func Render(c echo.Context, cmp templ.Component) error {
	//c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTML)
	return cmp.Render(c.Request().Context(), c.Response())
}

func setupRouter(e *echo.Echo) {
	e.RouteNotFound("/*", notFound)

	e.GET("/", index)
	e.GET("/me", author)
	e.GET("/author/:name", author)
	e.GET("/rename-author", authorRenameView)
	e.POST("/rename-author", authorRename)
	e.GET("/upload", uploadView)
	e.PUT("/upload", uploadFile)
	e.POST("/report/:id", reportRingtone)
	e.POST("/download/:id", downloadRingtone)
	e.GET("/rename/:id", renameView)
	e.POST("/rename/:id", rename)
	e.POST("/delete-ringtone/:id", deleteRingtone)
	e.GET("/guide", guide)
	e.GET("/dmca", dmca)
	e.GET("/google-login", googleLogin)
	e.GET("/google-callback", googleCallback)
	e.POST("/logout", logout)
}

func index(c echo.Context) error {
	authorID := utils.GetIDFromCookie(c)

	searchQuery := c.QueryParam("s")
	category, err := strconv.Atoi(c.QueryParam("c"))
	if err != nil {
		category = 0
	}
	sortBy := c.QueryParam("o")
	pageNumber, err := strconv.Atoi(c.QueryParam("page"))
	if err != nil {
		pageNumber = 1
	}
	includeAutoGenerated := c.QueryParam("gen") == "on"

	phonesMap := make(map[int]bool)
	effetsMap := make(map[int]bool)
	var phonesQuery []string = strings.Split(c.QueryParam("p"), ",")
	var effectsQuery []string = strings.Split(c.QueryParam("e"), ",")
	for _, v := range phonesQuery {
		phoneID, err := strconv.Atoi(v)
		if err == nil {
			phonesMap[phoneID] = true
		}
	}
	for _, v := range effectsQuery {
		effectID, err := strconv.Atoi(v)
		if err == nil {
			effetsMap[effectID] = true
		}
	}

	phonesArr := slices.Collect(maps.Keys(phonesMap))
	effectsArr := slices.Collect(maps.Keys(effetsMap))

	// if it is a htmx request, render only the new results
	if c.Request().Header.Get("HX-Request") == "true" {
		var ringtones []database.RingtoneModel
		var numberOfPages int

		ringtones, numberOfPages, err = database.GetRingtones(searchQuery, category, sortBy, phonesArr, effectsArr, includeAutoGenerated, pageNumber)
		if err != nil {
			return Render(c, views.OtherError(http.StatusInternalServerError, err))
		}
		c.Response().Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		c.Response().Header().Set("Pragma", "no-cache")
		return Render(c, components.ListOfRingtones(ringtones, numberOfPages, pageNumber, authorID == 1, "", "index"))
	}

	phones, err := database.GetPhones()
	if err != nil {
		return Render(c, views.OtherErrorView(http.StatusInternalServerError, err))
	}
	effects, err := database.GetEffects()
	if err != nil {
		return Render(c, views.OtherErrorView(http.StatusInternalServerError, err))
	}

	var ringtones []database.RingtoneModel
	var numberOfPages int

	ringtones, numberOfPages, err = database.GetRingtones(searchQuery, category, sortBy, phonesArr, effectsArr, includeAutoGenerated, pageNumber)
	if err != nil {
		return Render(c, views.OtherErrorView(http.StatusInternalServerError, err))
	}

	for i := range phones {
		phones[i].Selected = phonesMap[phones[i].ID]
	}
	for i := range effects {
		effects[i].Selected = effetsMap[effects[i].ID]
	}

	var data views.IndexData = views.IndexData{
		Ringtones:     ringtones,
		Phones:        phones,
		Effects:       effects,
		Category:      category,
		SortBy:        sortBy,
		SearchQuery:   searchQuery,
		AutoGenerated: includeAutoGenerated,
		NumberOfPages: numberOfPages,
		Page:          pageNumber,
		LoggedIn:      authorID != 0,
		Admin:         authorID == 1,
	}
	return Render(c, views.Index(data))
}

func author(c echo.Context) error {
	pageNumber, err := strconv.Atoi(c.QueryParam("page"))
	if err != nil {
		pageNumber = 1
	}

	var itsADifferentAuthor bool = true
	authorName := c.Param("name")

	userID := utils.GetIDFromCookie(c)

	var user database.AuthorModel
	if userID != 0 {
		user, err = database.GetAuthor(userID)
		if err != nil {
			if err == sql.ErrNoRows {
				utils.RemoveAuthCookie(c)
			}
			return Render(c, views.OtherErrorView(http.StatusInternalServerError, err))
		}

		if authorName == "" {
			authorName = user.Name
		}

		itsADifferentAuthor = user.Name != authorName
	}

	ringtones, numberOfPages, err := database.GetRingtonesByAuthor(authorName, pageNumber)
	if err != nil {
		return Render(c, views.OtherErrorView(http.StatusInternalServerError, err))
	}

	// if it is a htmx request, render only the new results
	if c.Request().Header.Get("HX-Request") == "true" {
		return Render(c, components.ListOfRingtones(ringtones, numberOfPages, pageNumber, !itsADifferentAuthor, authorName, "profile"))
	}

	var author database.AuthorModel
	if itsADifferentAuthor {
		author = database.AuthorModel{
			Name: authorName,
		}
	} else {
		author = user
	}

	_, err = c.Cookie(utils.CookieName)
	var data views.ProfileData = views.ProfileData{
		Ringtones:           ringtones,
		NumberOfPages:       numberOfPages,
		Page:                pageNumber,
		Author:              author,
		LoggedIn:            err == nil,
		ItsADifferentAuthor: itsADifferentAuthor,
	}
	return Render(c, views.Profile(data))
}

func renameView(c echo.Context) error {
	authorID := utils.GetIDFromCookie(c)
	if authorID == 0 {
		return Render(c, views.OtherErrorView(http.StatusBadRequest, errors.New("You're not logged in.")))
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	ringtone, err := database.GetRingtone(id)
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}

	return Render(c, components.Rename(ringtone, nil))
}

func rename(c echo.Context) error {
	authorID := utils.GetIDFromCookie(c)
	if authorID == 0 {
		return errors.New("You're not logged in.")
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	newName := c.FormValue("name")
	if !ringtoneNameR.MatchString(newName) {
		ringtone, err := database.GetRingtone(id)
		if err != nil {
			return Render(c, views.OtherErrorView(http.StatusInternalServerError, err))
		}
		ringtone.Name = newName
		return Render(c, components.Rename(ringtone, errors.New("The name must be 2-20 letters and only a-z and some special characters.")))
	}
	err = database.RenameRingtone(id, newName, authorID)
	if err != nil {
		ringtone, err := database.GetRingtone(id)
		if err != nil {
			return Render(c, views.OtherErrorView(http.StatusInternalServerError, err))
		}
		return Render(c, components.Rename(ringtone, errors.New("Something went wrong")))
	}
	ringtone, err := database.GetRingtone(id)
	if err != nil {
		return Render(c, components.Rename(ringtone, errors.New("Something went wrong")))
	}

	return Render(c, components.Captions(ringtone, true))
}

func authorRenameView(c echo.Context) error {
	authorID := utils.GetIDFromCookie(c)
	if authorID == 0 {
		return Render(c, views.OtherErrorView(http.StatusBadRequest, errors.New("You're not logged in.")))
	}
	author, err := database.GetAuthor(authorID)
	if err != nil {
		return Render(c, views.OtherErrorView(http.StatusInternalServerError, err))
	}

	return Render(c, components.EditName(author.Name))
}

func authorRename(c echo.Context) error {
	authorID := utils.GetIDFromCookie(c)
	if authorID == 0 {
		return errors.New("You're not logged in.")
	}
	newName := strings.Trim(c.FormValue("name"), " ")
	if !authorNameR.MatchString(newName) {
		return Render(c, views.OtherError(http.StatusInternalServerError, errors.New("Invalid name. Maximal length is 20 letters. Only ASCII characters are allowed (a-z and some special characters).")))
	}
	email, err := database.RenameAuthor(authorID, newName)
	if err != nil {
		return Render(c, views.OtherError(http.StatusInternalServerError, errors.New("Something went wrong")))
	}

	return Render(c, components.AuthorProfile(newName, email))
}

func uploadView(c echo.Context) error {
	effects, err := database.GetEffects()
	if err != nil {
		return Render(c, views.OtherErrorView(http.StatusInternalServerError, err))
	}

	_, err = c.Cookie(utils.CookieName)
	return Render(c, views.Upload(err == nil, c.FormValue("c"), effects, "", "", nil))
}

func uploadFile(c echo.Context) error {
	authorID := utils.GetIDFromCookie(c)
	if authorID == 0 {
		return Render(c, views.OtherError(http.StatusBadRequest, errors.New("Only logged-in authorors can upload Glyphtones")))
	}
	author, err := database.GetAuthor(authorID)
	if err != nil {
		return Render(c, views.OtherError(http.StatusInternalServerError, errors.New("Something went wrong")))
	}

	errorHandler := func(mainErr error) error {
		effects, err := database.GetEffects()
		if err != nil {
			return Render(c, views.OtherErrorView(http.StatusInternalServerError, err))
		}
		return Render(c, views.UploadForm(c.FormValue("c"), effects, c.FormValue("e"), c.FormValue("name"), authorID != 0, mainErr))
	}

	if author.Banned {
		log.Println("ban")
		return errorHandler(errors.New("You cannot upload since you are banned!"))
	}

	name := c.FormValue("name")
	if !ringtoneNameR.MatchString(name) {
		return errorHandler(errors.New("Name must be 2-30 characters long and without diacritics."))
	}
	category, err1 := strconv.Atoi(c.FormValue("c"))
	effect, err2 := strconv.Atoi(c.FormValue("e"))
	if err1 != nil || err2 != nil || c.FormValue("gen") == "" {
		return errorHandler(errors.New("Missing form values."))
	}
	autoGenerated := c.FormValue("gen") == "true"
	file, err := c.FormFile("ringtone")
	if err != nil {
		return errorHandler(errors.New("Missing the file."))
	}
	split := strings.Split(file.Filename, ".")
	if len(split) == 0 || split[len(split)-1] != "ogg" {
		return errorHandler(errors.New("It seems that the file provided is not a Nothing Glyphtone."))
	}

	src, err := file.Open()
	if err != nil {
		return Render(c, views.OtherError(http.StatusInternalServerError, err))
	}
	defer src.Close()

	tmpFile, err := utils.CreateTemporaryFile(src)
	if err != nil {
		log.Println(err)
		return Render(c, views.OtherError(http.StatusInternalServerError, err))
	}
	defer func() {
		name := tmpFile.Name()
		tmpFile.Close()
		utils.DeleteFile(name)
	}()

	stats, err := os.Stat(tmpFile.Name())
	if err != nil {
		return Render(c, views.OtherError(http.StatusInternalServerError, err))
	}
	if stats.Size() > maxRingtoneSize {
		return errorHandler(errors.New("The file is too large! (3MB limit)"))
	}

	phones, err := database.GetPhones()
	if err != nil {
		return Render(c, views.OtherErrorView(http.StatusInternalServerError, err))
	}

	phonesCompatibleIDs, glyphData, ok := utils.CheckFile(tmpFile, phones)
	if !ok {
		return errorHandler(errors.New("It seems that the file provided is not a Nothing Glyphtone."))
	}

	ringtoneID, err := database.CreateRingtone(name, category, phonesCompatibleIDs, effect, authorID, autoGenerated, glyphData)
	if err != nil {
		return Render(c, views.OtherError(http.StatusInternalServerError, err))
	}
	err = utils.CreateRingtoneFile(tmpFile, ringtoneID)
	if err != nil {
		return Render(c, views.OtherError(http.StatusInternalServerError, err))
	}

	return Render(c, views.SuccessfulUpload())
}

func reportRingtone(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	_, err = c.Cookie(fmt.Sprintf("Glyphtone_%d_reported", id))
	if err == nil {
		// already reported this glyphtone
		return c.NoContent(http.StatusOK)
	}

	err = database.RingtoneIncreaseNotWorking(id)
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}
	cookie := http.Cookie{
		Name:    fmt.Sprintf("Glyphtone_%d_reported", id),
		Value:   "true",
		Expires: time.Now().Add(time.Hour * 1_000_000), // ~ 100 years
		Path:    "/",
	}
	c.SetCookie(&cookie)
	return c.NoContent(http.StatusOK)
}

func downloadRingtone(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	_, err = c.Cookie(fmt.Sprintf("Glyphtone_%d_downloaded", id))
	if err == nil {
		// already downloaded this glyphtone
		return c.NoContent(http.StatusOK)
	}
	err = database.RingtoneIncreaseDownload(id)
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}
	cookie := http.Cookie{
		Name:    fmt.Sprintf("Glyphtone_%d_downloaded", id),
		Value:   "true",
		Expires: time.Now().Add(time.Hour * 1_000_000), // ~ 100 years
		Path:    "/",
	}
	c.SetCookie(&cookie)
	return c.NoContent(http.StatusOK)
}

func deleteRingtone(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	authorID := utils.GetIDFromCookie(c)
	if authorID == 0 {
		return Render(c, views.OtherError(http.StatusBadRequest, errors.New("You're not logged in.")))
	}

	err = database.DeleteRingtone(id, authorID)
	if err != nil {
		return Render(c, views.OtherError(http.StatusBadRequest, err))
	}

	utils.DeleteFile(fmt.Sprintf("%s/%d.ogg", utils.RingtonesDir, id))

	c.Response().Header().Set("HX-Refresh", "true")
	return c.NoContent(http.StatusOK)
}

func guide(c echo.Context) error {
	_, err := c.Cookie(utils.CookieName)
	return Render(c, views.Guide(err == nil))
}

func dmca(c echo.Context) error {
	_, err := c.Cookie(utils.CookieName)
	return Render(c, views.Dmca(err == nil))
}

func googleLogin(c echo.Context) error {
	url := googleOauthConfig.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	return c.Redirect(http.StatusTemporaryRedirect, url)
}

func googleCallback(c echo.Context) error {
	// get the authorization code from the query parameters
	code := c.QueryParam("code")
	if code == "" {
		return Render(c, views.OtherErrorView(http.StatusBadRequest, errors.New("Bad request")))
	}

	// exchange the code for a token
	token, err := googleOauthConfig.Exchange(context.Background(), code)
	if err != nil {
		log.Println(err)
		return Render(c, views.OtherErrorView(http.StatusInternalServerError, errors.New("Failed to exchange token")))
	}

	// use the token to get user information
	client := googleOauthConfig.Client(context.Background(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return Render(c, views.OtherErrorView(http.StatusInternalServerError, errors.New("Failed to fetch user info")))
	}
	defer resp.Body.Close()

	// decode the user information
	var authorInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&authorInfo); err != nil {
		return Render(c, views.OtherErrorView(http.StatusInternalServerError, errors.New("Failed to decode author info")))
	}

	name := authorInfo["name"].(string)
	name = godiacritics.Normalize(name)
	name = strings.Trim(name, " ")
	if len(name) > 30 {
		name = name[0:30]
	}
	if !authorNameR.MatchString(name) {
		name = fmt.Sprintf("Author%d", rand.IntN(10000))
	}

	authorID, err := database.CreateAuthor(name, authorInfo["email"].(string))
	if err != nil {
		if strings.Contains(err.Error(), "unique_name") {
			authorID, _ = database.CreateAuthor(fmt.Sprintf("%s%d", name, rand.IntN(10000)), authorInfo["email"].(string))
		} else {
			return Render(c, views.OtherErrorView(http.StatusInternalServerError, err))
		}
	}

	utils.WriteAuthCookie(c, authorID)
	return c.Redirect(http.StatusTemporaryRedirect, "/me")
}

func logout(c echo.Context) error {
	utils.RemoveAuthCookie(c)
	c.Response().Header().Set("HX-Redirect", "/")
	return Render(c, components.Header(false))
}

func notFound(c echo.Context) error {
	_, err := c.Cookie(utils.CookieName)
	return Render(c, views.NotFoundView(err == nil))
}
