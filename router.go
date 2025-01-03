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
	e.GET("/author", author)
	e.GET("/author/:id", author)
	e.GET("/rename-author", authorRenameView)
	e.POST("/rename-author", authorRename)
	e.GET("/upload", uploadView)
	e.PUT("/upload", uploadFile)
	e.POST("/report/:id", reportRingtone)
	e.POST("/download/:id", downloadRingtone)
	e.POST("/delete-ringtone/:id", deleteRingtone)
	e.GET("/guide", guide)
	e.GET("/google-login", googleLogin)
	e.GET("/google-callback", googleCallback)
	e.POST("/logout", logout)
}

func index(c echo.Context) error {
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

		ringtones, numberOfPages, err = database.GetRingtones(searchQuery, category, sortBy, phonesArr, effectsArr, pageNumber)
		if err != nil {
			return Render(c, views.OtherError(http.StatusInternalServerError, err))
		}
		return Render(c, components.ListOfRingtones(ringtones, numberOfPages, pageNumber, false, 0, "index"))
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

	ringtones, numberOfPages, err = database.GetRingtones(searchQuery, category, sortBy, phonesArr, effectsArr, pageNumber)
	if err != nil {
		return Render(c, views.OtherErrorView(http.StatusInternalServerError, err))
	}

	for i := range phones {
		phones[i].Selected = phonesMap[phones[i].ID]
	}
	for i := range effects {
		effects[i].Selected = effetsMap[effects[i].ID]
	}

	_, err = c.Cookie(utils.CookieName)
	var data views.IndexData = views.IndexData{
		Ringtones:     ringtones,
		Phones:        phones,
		Effects:       effects,
		Category:      category,
		SortBy:        sortBy,
		SearchQuery:   searchQuery,
		NumberOfPages: numberOfPages,
		Page:          pageNumber,
		LoggedIn:      err == nil,
	}
	return Render(c, views.Index(data))
}

func author(c echo.Context) error {
	pageNumber, err := strconv.Atoi(c.QueryParam("page"))
	if err != nil {
		pageNumber = 1
	}

	var itsADifferentAuthor bool
	authorStr := c.Param("id")
	var authorID int
	if authorStr != "" {
		authorID, err = strconv.Atoi(authorStr)
		if err != nil {
			return Render(c, views.OtherErrorView(http.StatusBadRequest, errors.New("Bad url.")))
		}
		loggedInAuthorID := utils.GetIDFromCookie(c)

		itsADifferentAuthor = authorID != loggedInAuthorID
	} else {
		authorID = utils.GetIDFromCookie(c)
		if authorID == 0 {
			return Render(c, views.OtherErrorView(http.StatusBadRequest, errors.New("You're not logged in.")))
		}
		itsADifferentAuthor = false
	}

	author, err := database.GetAuthor(authorID)
	if err != nil {
		if err == sql.ErrNoRows {
			utils.RemoveAuthCookie(c)
		}
		return Render(c, views.OtherErrorView(http.StatusInternalServerError, err))
	}

	ringtones, numberOfPages, err := database.GetRingtonesByAuthor(authorID, pageNumber)
	if err != nil {
		return Render(c, views.OtherErrorView(http.StatusInternalServerError, err))
	}

	// if it is a htmx request, render only the new results
	if c.Request().Header.Get("HX-Request") == "true" {
		return Render(c, components.ListOfRingtones(ringtones, numberOfPages, pageNumber, !itsADifferentAuthor, author.ID, "profile"))
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
	newName := c.FormValue("name")
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

	phones, err := database.GetPhones()
	if err != nil {
		return Render(c, views.OtherErrorView(http.StatusInternalServerError, err))
	}

	errorHandler := func(mainErr error) error {
		effects, err := database.GetEffects()
		if err != nil {
			return Render(c, views.OtherErrorView(http.StatusInternalServerError, err))
		}
		return Render(c, views.UploadForm(c.FormValue("c"), effects, c.FormValue("e"), c.FormValue("name"), authorID != 0, mainErr))
	}

	name := c.FormValue("name")
	if !ringtoneNameR.MatchString(name) {
		return errorHandler(errors.New("Name must be 2-30 characters long and without diacritics."))
	}
	category, err1 := strconv.Atoi(c.FormValue("c"))
	effect, err2 := strconv.Atoi(c.FormValue("e"))
	if err1 != nil || err2 != nil {
		return errorHandler(errors.New("Missing form values."))
	}
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
		return errorHandler(errors.New("The file is too large! (2MB limit)"))
	}

	phonesCompatibleIDs, ok := utils.CheckFile(tmpFile, phones)
	if !ok {
		return errorHandler(errors.New("It seems that the file provided is not a Nothing Glyphtone."))
	}

	ringtoneID, err := database.CreateRingtone(name, category, phonesCompatibleIDs, effect, authorID)
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
	err = database.RingtoneIncreaseDownload(id)
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}
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
	if len(name) > 30 {
		name = name[0:30]
	}
	if !authorNameR.MatchString(name) {
		name = fmt.Sprintf("Author%d", rand.IntN(10000))
	}

	authorID, err := database.CreateAuthor(name, authorInfo["email"].(string))
	if err != nil {
		return Render(c, views.OtherErrorView(http.StatusInternalServerError, err))
	}

	utils.WriteAuthCookie(c, authorID)
	return c.Redirect(http.StatusTemporaryRedirect, "/")
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
