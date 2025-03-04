package database

import (
	"database/sql"

	"github.com/lib/pq"
	"github.com/rickb777/date/v2"
)

type RingtoneModel struct {
	ID              int            `db:"id"`
	Name            string         `db:"name"`
	PhoneNames      pq.StringArray `db:"phone_names"`
	EffectName      string         `db:"effect_name"`
	Category        int            `db:"category"`
	Downloads       int            `db:"downloads"`
	AuthorName      string         `db:"author_name"`
	AuthorID        int            `db:"author_id"`
	NumberOfResults int            `db:"results"`
	Score           float32        `db:"score"`
	Glyphs          sql.NullString `db:"glyphs"` // converted to base64 and zlib compressed
}

type PhoneModel struct {
	ID               int    `db:"id"`
	Name             string `db:"name"`
	NumberOfColumns  int    `db:"cols"`
	NumberOfColumns2 int    `db:"cols2"`
	Selected         bool   `db:""`
}

type EffectModel struct {
	ID       int    `db:"id"`
	Name     string `db:"name"`
	Selected bool   `db:""`
}

type AuthorModel struct {
	ID         int       `db:"id"`
	Name       string    `db:"name"`
	Email      string    `db:"email"`
	DateJoined date.Date `db:"date_joined"`
}
