package stream

import (
	"errors"
	"reflect"
	"sort"
)

type stream struct {
	ops  []op
	data []interface{}
}

type op struct {
	typ string
	fun reflect.Value
}

type sortbyfun struct {
	data []interface{}
	fun  reflect.Value
}

func (s sortbyfun) Len() int            { return len(s.data) }
func (s *sortbyfun) Swap(i, j int)      { s.data[i], s.data[j] = s.data[j], s.data[i] }
func (s *sortbyfun) Less(i, j int) bool { return call(s.fun, s.data[i], s.data[j])[0].Bool() }

func New(arr interface{}) (*stream, error) {
	ops := make([]op, 0)
	data := make([]interface{}, 0)
	dataValue := reflect.ValueOf(&data).Elem()
	arrValue := reflect.ValueOf(arr)
	if arrValue.Kind() == reflect.Ptr {
		arrValue = arrValue.Elem()
	}
	if arrValue.Kind() == reflect.Slice || arrValue.Kind() == reflect.Array {
		for i := 0; i < arrValue.Len(); i++ {
			dataValue.Set(reflect.Append(dataValue, arrValue.Index(i)))
		}
	} else {
		return nil, errors.New("the type of arr parameter must be Array or Slice")
	}
	return &stream{ops: ops, data: data}, nil
}

func Of(args ...interface{}) (*stream, error) {
	return New(args)
}

func Ints(args ...int64) (*stream, error) {
	return New(args)
}

func Floats(args ...float64) (*stream, error) {
	return New(args)
}

func Strings(args ...string) (*stream, error) {
	return New(args)
}

func (s *stream) Reset() *stream {
	s.ops = make([]op, 0)
	return s
}

//  Filter operation. filterFunc: func(o T) bool
func (s *stream) Filter(filterFunc interface{}) *stream {
	funcValue := reflect.ValueOf(filterFunc)
	s.ops = append(s.ops, op{typ: "filter", fun: funcValue})
	return s
}

//  Map operation. mapFunc: func(o T1) T2
func (s *stream) Map(mapFunc interface{}) *stream {
	funcValue := reflect.ValueOf(mapFunc)
	s.ops = append(s.ops, op{typ: "map", fun: funcValue})
	return s
}

// Sort operation. lessFunc: func(o1,o2 T) bool
func (s *stream) Sort(lessFunc interface{}) *stream {
	funcValue := reflect.ValueOf(lessFunc)
	s.ops = append(s.ops, op{typ: "sort", fun: funcValue})
	return s
}

// Distinct operation. equalFunc: func(o1,o2 T) bool
func (s *stream) Distinct(equalFunc interface{}) *stream {
	funcValue := reflect.ValueOf(equalFunc)
	s.ops = append(s.ops, op{typ: "distinct", fun: funcValue})
	return s
}

// Peek operation. peekFunc: func(o T)
func (s *stream) Peek(peekFunc interface{}) *stream {
	funcValue := reflect.ValueOf(peekFunc)
	s.ops = append(s.ops, op{typ: "peek", fun: funcValue})
	return s
}

// Limit operation.
func (s *stream) Limit(num int) *stream {
	if num < 0 {
		num = 0
	}
	funcValue := reflect.ValueOf(func() int { return num })
	s.ops = append(s.ops, op{typ: "limit", fun: funcValue})
	return s
}

// Skip operation.
func (s *stream) Skip(num int) *stream {
	if num < 0 {
		num = 0
	}
	funcValue := reflect.ValueOf(func() int { return num })
	s.ops = append(s.ops, op{typ: "skip", fun: funcValue})
	return s
}

// Collect operation.
func (s *stream) Collect() []interface{} {
	result := s.data
	for _, op := range s.ops {
		if len(result) == 0 {
			break
		}
		switch op.typ {
		case "filter":
			temp := make([]interface{}, 0)
			each(result, op.fun, func(i int, it interface{}, out []reflect.Value) bool {
				if out[0].Bool() {
					temp = append(temp, it)
				}
				return true
			})
			result = temp
		case "peek":
			each(result, op.fun, emptyeachfunc)
		case "map":
			temp := make([]interface{}, 0)
			tempVlaue := reflect.ValueOf(&temp).Elem()
			each(result, op.fun, func(i int, it interface{}, out []reflect.Value) bool {
				tempVlaue.Set(reflect.Append(tempVlaue, out[0]))
				return true
			})
			result = temp
		case "sort":
			sort.Sort(&sortbyfun{data: result, fun: op.fun})
		case "distinct":
			temp := make([]interface{}, 0)
			temp = append(temp, result[0])
			for _, it := range result {
				found := false
				for _, it2 := range temp {
					out := call(op.fun, it, it2)
					if out[0].Bool() {
						found = true
					}
				}
				if !found {
					temp = append(temp, it)
				}
			}
			result = temp
		case "limit":
			limit := int(call(op.fun)[0].Int())
			if limit > len(result) {
				limit = len(result)
			}
			temp := result
			result = temp[:limit]
		case "skip":
			skip := int(call(op.fun)[0].Int())
			if skip > len(result) {
				skip = len(result)
			}
			temp := result
			result = temp[skip:]
		}
	}
	return result
}

// Exec operation.
func (s *stream) Exec() {
	s.Collect()
}

// ForEach operation. actFunc: func(o T)
func (s *stream) ForEach(actFunc interface{}) {
	data := s.Collect()
	each(data, reflect.ValueOf(actFunc), emptyeachfunc)
}

// AllMatch operation. matchFunc: func(o T) bool
func (s *stream) AllMatch(matchFunc interface{}) bool {
	data := s.Collect()
	allMatch := true
	each(data, reflect.ValueOf(matchFunc), func(i int, it interface{}, out []reflect.Value) bool {
		if !out[0].Bool() {
			allMatch = false
			return false
		}
		return true
	})
	return allMatch
}

// AnyMatch operation. matchFunc: func(o T) bool
func (s *stream) AnyMatch(matchFunc interface{}) bool {
	data := s.Collect()
	anyMatch := false
	each(data, reflect.ValueOf(matchFunc), func(i int, it interface{}, out []reflect.Value) bool {
		if out[0].Bool() {
			anyMatch = true
			return false
		}
		return true
	})
	return anyMatch
}

// NoneMatch operation. matchFunc: func(o T) bool
func (s *stream) NoneMatch(matchFunc interface{}) bool {
	data := s.Collect()
	noneMatch := true
	each(data, reflect.ValueOf(matchFunc), func(i int, it interface{}, out []reflect.Value) bool {
		if out[0].Bool() {
			noneMatch = false
			return false
		}
		return true
	})
	return noneMatch
}

// Count operation.
func (s *stream) Count() int {
	return len(s.Collect())
}

// Max operation.lessFunc: func(o1,o2 T) bool
func (s *stream) Max(lessFunc interface{}) interface{} {
	funcValue := reflect.ValueOf(lessFunc)
	data := s.Collect()
	var max interface{}
	if len(data) > 0 {
		max = data[0]
		for i := 1; i < len(data); i++ {
			out := call(funcValue, max, data[i])
			if out[0].Bool() {
				max = data[i]
			}
		}
	}
	return max
}

// Min operation.lessFunc: func(o1,o2 T) bool
func (s *stream) Min(lessFunc interface{}) interface{} {
	funcValue := reflect.ValueOf(lessFunc)
	data := s.Collect()
	var min interface{}
	if len(data) > 0 {
		min = data[0]
		for i := 1; i < len(data); i++ {
			out := call(funcValue, data[i], min)
			if out[0].Bool() {
				min = data[i]
			}
		}
	}
	return min
}

// First operation. matchFunc: func(o T) bool
func (s *stream) First(matchFunc interface{}) interface{} {
	data := s.Collect()
	funcValue := reflect.ValueOf(matchFunc)
	for _, it := range data {
		out := call(funcValue, it)
		if out[0].Bool() {
			return it
		}
	}
	return nil
}

// Last operation. matchFunc: func(o T) bool
func (s *stream) Last(matchFunc interface{}) interface{} {
	data := s.Collect()
	funcValue := reflect.ValueOf(matchFunc)
	for i := len(data) - 1; i >= 0; i-- {
		it := data[i]
		out := call(funcValue, it)
		if out[0].Bool() {
			return it
		}
	}
	return nil
}

// Reduce operation. reduceFunc: func(r T2,o T) T2
func (s *stream) Reduce(initValue interface{}, reduceFunc interface{}) interface{} {
	data := s.Collect()
	funcValue := reflect.ValueOf(reduceFunc)
	result := initValue
	rValue := reflect.ValueOf(&result).Elem()
	for _, it := range data {
		out := call(funcValue, result, it)
		rValue.Set(out[0])
	}
	return result
}

//  eachfunc is the function for each method,return if should continue loop
type eachfunc func(int, interface{}, []reflect.Value) bool

//  emptyeachfunc the empty eachfunc, return true
var emptyeachfunc = func(int, interface{}, []reflect.Value) bool { return true }

func each(data []interface{}, fun reflect.Value, act eachfunc) {
	for i, it := range data {
		out := call(fun, it)
		if !act(i, it, out) {
			break
		}
	}
}

func call(fun reflect.Value, args ...interface{}) []reflect.Value {
	in := make([]reflect.Value, len(args))
	for i, a := range args {
		in[i] = reflect.ValueOf(a)
	}
	return fun.Call(in)
}