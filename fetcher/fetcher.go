package fetcher

/*
import (
	"rasp_info/config"
	"rasp_info/store"
)
*/

type Fetcher interface {
	Fetch() error
}
