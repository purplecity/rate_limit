package rate_limit

import (
	"runtime"
	"sync"
	"time"
)

// --------------------------------bucket-----------------------//
const (
	ONE_SECOND = 1000000000
)

type Bucket struct {
	TokenRemain  int32
	LastSyncTime int64
	Mx           *sync.Mutex
}

func (b *Bucket) resync(filter_key string, c *Rule) (int32, int64) {
	now := time.Now().UnixNano()
	b.Mx.Lock()
	defer b.Mx.Unlock()

	if b.LastSyncTime == 0 {
		b.TokenRemain = c.Limit
		b.LastSyncTime = now
		if c.Limit > 0 {
			b.TokenRemain--
		}
		return c.Limit, b.LastSyncTime
	}

	if now > b.LastSyncTime {
		tmp := ((now - b.LastSyncTime) / (int64(c.Duration) * ONE_SECOND)) * int64(c.Limit)

		if tmp > 0 {
			b.TokenRemain = b.TokenRemain + int32(tmp)
			if b.TokenRemain > c.Limit {
				b.TokenRemain = c.Limit
			}
			b.LastSyncTime = now
		}
	}

	// Consume one token
	before := b.TokenRemain
	if before > 0 {
		b.TokenRemain--
	}

	return before, b.LastSyncTime
}

// --------------------------------rule-----------------------//
// The Rule of one uri. the same user can just access this uri 'Limit' times in 'Duration' time
type Rule struct {
	Limit    int32
	Duration int32 // Units per second
}

type Rules map[string][]*Rule

func (rules Rules) AddRule(pattern string, rule *Rule) {
	if _, ok := rules[pattern]; !ok {
		rules[pattern] = make([]*Rule, 0)
	}

	rules[pattern] = append(rules[pattern], rule)
}

func NewRules() Rules {
	return make(Rules)
}

// --------------------------------meme rate limit-----------------------//

type MemRateLimiter struct {
	// filiter store the status of access for every user
	filter map[string]map[string][]*Bucket
	// rules'key is the uri that need frequency verification, and the rules'value is the Rules of uri(see Rule struct)
	rules Rules

	mx sync.RWMutex
}

func (r *MemRateLimiter) TokenAccess(filter_key string, pattern string) bool {
	buckets := r.getBuckets(filter_key, pattern)
	if len(buckets) == 0 {
		// No rate limit rules, allow access
		return true
	}

	record_map.Set(filter_key, time.Now().Unix()) // This filter_key has been accessed, buckets for all rules under this pattern have been generated
	for i, b := range buckets {
		// Resync token quantity
		remain, _ := b.resync(filter_key, r.rules[pattern][i])
		if remain <= 0 {
			return false
		}
	}

	// All rules passed
	return true
}

func (r *MemRateLimiter) getBuckets(filter_key string, pattern string) []*Bucket {
	r.mx.Lock()
	defer r.mx.Unlock()

	if len(r.rules[pattern]) == 0 {
		// No rules, no rate limiting needed
		return []*Bucket{}
	}

	// Initialize the map for the filter key
	if r.filter[filter_key] == nil {
		r.filter[filter_key] = make(map[string][]*Bucket)
	}

	// If there are no buckets under this pattern, create buckets
	if len(r.filter[filter_key][pattern]) == 0 {
		for i := 0; i < len(r.rules[pattern]); i++ {
			bucket := Bucket_Get()
			r.filter[filter_key][pattern] = append(r.filter[filter_key][pattern], bucket)
		}
	}

	return r.filter[filter_key][pattern]
}

func newMemRateLimiter() *MemRateLimiter {
	r := new(MemRateLimiter)
	r.filter = make(map[string]map[string][]*Bucket)
	r.rules = NewRules()
	r.mx = sync.RWMutex{}
	return r
}

var (
	mem_rate_limiter *MemRateLimiter
	record_map       *StringInt64Map
)

func AppAddRule(pattern string, limit int32, duration int32) {
	mem_rate_limiter.rules.AddRule(pattern, &Rule{
		Limit:    limit,
		Duration: duration,
	})
}

func AppTokenAccess(key, pattern string) bool {
	return mem_rate_limiter.TokenAccess(key, pattern)
}

// -----------------pool---------------------//

type _Bucket_Pool struct {
	p *sync.Pool
}

var pool_Bucket *_Bucket_Pool

var once_Bucket sync.Once

func get_Bucket() *_Bucket_Pool {
	once_Bucket.Do(func() {
		pool_Bucket = &_Bucket_Pool{p: &sync.Pool{
			New: func() interface{} {
				return &Bucket{
					TokenRemain:  0,
					LastSyncTime: 0,
					Mx:           &sync.Mutex{},
				}
			},
		}}
	})
	return pool_Bucket
}

func Bucket_Get() *Bucket {
	p := get_Bucket()
	return p.p.Get().(*Bucket)
}

// Keep the mutex unchanged. If it's assigned nil here, then Bucket_Get needs to check if it's empty and assign a new lock if needed
func Bucket_Put(b *Bucket) {
	p := get_Bucket()
	b.LastSyncTime = 0
	b.TokenRemain = 0
	p.p.Put(b)
}

func Init_rate_limit(minutes, check_uid_num int) {
	record_map = NewStringInt64Map()
	mem_rate_limiter = newMemRateLimiter()
	go Start_rate_limit(minutes, check_uid_num)
}

// Check check_uid_num items every minutes minutes
func Start_rate_limit(minutes, check_uid_num int) {
	defer func() {
		err := recover()
		if err != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]
		}
	}()

	expire_time := int64(30 * 60) // 30 minutes
	t := time.Tick(time.Minute * time.Duration(minutes))
	for range t {
		now := time.Now().Unix()
		delete_keys := []string{}
		record_items := record_map.SomeItems(check_uid_num)
		// No access to this filter_key within expire_time
		for filter_key, ts := range record_items {
			if ts+expire_time < now {
				delete_keys = append(delete_keys, filter_key)
			}
		}
		if len(delete_keys) > 0 {
			mem_rate_limiter.mx.Lock()
			for _, e := range delete_keys { // Remove the filter_key directly
				ee := mem_rate_limiter.filter[e]
				// Delete all buckets corresponding to the pattern
				for _, e_list := range ee {
					for _, eee := range e_list {
						Bucket_Put(eee)
					}
				}
				delete(mem_rate_limiter.filter, e)
				clear(ee)
				ee = nil

			}
			mem_rate_limiter.mx.Unlock()
		}

		// Also delete the corresponding entry in the record map
		for _, e := range delete_keys {
			record_map.Delete(e)
		}

		clear(delete_keys)
		delete_keys = nil
		clear(record_items)
		record_items = nil
	}
}
