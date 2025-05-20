package shared

type MediaInfo struct {
	Id          string
	Url         string
	UrlSign     string
	CoverUrl    string
	Size        string
	Domain      string
	Classify    string
	Suffix      string
	SavePath    string
	Status      string
	DecodeKey   string
	Description string
	ContentType string
	OtherData   map[string]string
}
