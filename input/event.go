package input

import (
	"time"

	"github.com/xjayleex/minari-libs/thirdparty/mapstr"
)

type Event struct {
	Timestamp  time.Time
	Metadata   mapstr.M
	Fields     mapstr.M
	Private    interface{}
	TimeSeries bool
}
