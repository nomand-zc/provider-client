package limitrule

import (
	"fmt"
	"time"
)

// TimeGranularity 表示时间窗口的粒度/周期类型。
type TimeGranularity string

// String returns the string representation of the time granularity.
func (r TimeGranularity) String() string {
	return string(r)
}

// IsValid checks if the time granularity is one of the defined constants.
func (r TimeGranularity) IsValid() bool {
	switch r {
	case GranularityMinute, GranularityHour,
		GranularityDay, GranularityWeek,
		GranularityMonth:
		return true
	default:
		return false
	}
}

const (
	GranularityMinute TimeGranularity = "minute"
	GranularityHour   TimeGranularity = "hour"
	GranularityDay    TimeGranularity = "day"
	GranularityWeek   TimeGranularity = "week"
	GranularityMonth  TimeGranularity = "month"
)

type SourceType int

const (
	// SourceTypeToken token数量/积分数量
	SourceTypeToken SourceType = 1
	// SourceTypeRequest 请求次数
	SourceTypeRequest SourceType = 2
)

// LimitRule 限流规则
type LimitRule struct {
	SourceType      SourceType      `json:"source_type"`
	TimeGranularity TimeGranularity `json:"time_granularity"`
	// WindowSize 窗口大小,单位为时间粒度。
	// 比如时间粒度为分钟，窗口大小为10，则窗口为10分钟。
	WindowSize int `json:"window_size"`
	// Total 总量
	Total float64 `json:"total"`
	// Used 已使用量
	Used float64 `json:"used"`
	// Remain 剩余量
	Remain float64 `json:"remain"`

	// StartTime 窗口开始时间
	StartTime *time.Time `json:"start_time"`
	// EndTime 窗口结束时间
	EndTime *time.Time `json:"end_time"`
}

// IsEmpty 判断规则是否为空
func (r *LimitRule) IsEmpty() bool {
	return r == nil || r.Total == 0
}

// String 返回规则字符串
func (r *LimitRule) String() string {
	return fmt.Sprintf("source_type: %d, time_granularity: %s, window_size: %d, total: %f, used: %f, remain: %f, start_time: %v, end_time: %v",
		r.SourceType, r.TimeGranularity, r.WindowSize, r.Total, r.Used, r.Remain, r.StartTime, r.EndTime)
}

// Clone 克隆规则
func (r *LimitRule) Clone() *LimitRule {
	return &LimitRule{
		SourceType:      r.SourceType,
		TimeGranularity: r.TimeGranularity,
		WindowSize:      r.WindowSize,
		Total:           r.Total,
		Used:            r.Used,
		Remain:          r.Remain,
		StartTime:       r.StartTime,
		EndTime:         r.EndTime,
	}
}

// 判断当前时间是否在窗口内
func (r *LimitRule) IsInWindow() bool {
	t := time.Now()
	if r.StartTime == nil || r.EndTime == nil {
		return false
	}
	return t.After(*r.StartTime) && t.Before(*r.EndTime)
}

// 根据当前时间，计算窗口开始时间和结束时间
func (r *LimitRule) CalculateWindowTime() {
	now := time.Now()
	var start time.Time

	switch r.TimeGranularity {
	case GranularityMinute:
		// 对齐到分钟起始
		start = time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), 0, 0, now.Location())
		end := start.Add(time.Duration(r.WindowSize) * time.Minute)
		r.StartTime = &start
		r.EndTime = &end
	case GranularityHour:
		// 对齐到小时起始
		start = time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
		end := start.Add(time.Duration(r.WindowSize) * time.Hour)
		r.StartTime = &start
		r.EndTime = &end
	case GranularityDay:
		// 对齐到天起始
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		end := start.AddDate(0, 0, r.WindowSize)
		r.StartTime = &start
		r.EndTime = &end
	case GranularityWeek:
		// 对齐到本周一起始
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7 // 将周日从0改为7，使周一为1
		}
		start = time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
		end := start.AddDate(0, 0, r.WindowSize*7)
		r.StartTime = &start
		r.EndTime = &end
	case GranularityMonth:
		// 对齐到月起始
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		end := start.AddDate(0, r.WindowSize, 0)
		r.StartTime = &start
		r.EndTime = &end
	}
}

// 计算下一个周期的开始时间和结束时间
func (r *LimitRule) CalculateNextWindowTime() (*time.Time, *time.Time) {
	// 若当前窗口尚未计算，先计算当前窗口
	if r.EndTime == nil {
		r.CalculateWindowTime()
	}
	if r.EndTime == nil {
		return nil, nil
	}

	// 以当前窗口结束时间作为下一个窗口的起始点
	nextStart := *r.EndTime
	var nextEnd time.Time

	switch r.TimeGranularity {
	case GranularityMinute:
		nextEnd = nextStart.Add(time.Duration(r.WindowSize) * time.Minute)
	case GranularityHour:
		nextEnd = nextStart.Add(time.Duration(r.WindowSize) * time.Hour)
	case GranularityDay:
		nextEnd = nextStart.AddDate(0, 0, r.WindowSize)
	case GranularityWeek:
		nextEnd = nextStart.AddDate(0, 0, r.WindowSize*7)
	case GranularityMonth:
		nextEnd = nextStart.AddDate(0, r.WindowSize, 0)
	default:
		return nil, nil
	}

	return &nextStart, &nextEnd
}
