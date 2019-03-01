package mydsl

import (
	"errors"
	"fmt"
	"gopkg.in/yaml.v2"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Argument struct {
	rawArg interface{}
}

func NewArgument(any interface{}) Argument {
	switch value := any.(type) {
	case string:
		anyString := value
		if anyString == "$" {
			return Argument{"$"}
		} else {
			return Argument{dollerReplacePattern.ReplaceAllString(anyString, "$.")}
		}
	default:
		return Argument{value}
	}
}

func toString(any interface{}) string {
	switch value := any.(type) {
	case string:
		return value
	case int:
		return strconv.Itoa(value)
	case float64:
		return strconv.Itoa(int(value))
	}
	return ""
}

func toInt(any interface{}) (int, error) {
	switch value := any.(type) {
	case string:
		num, numNg := strconv.Atoi(value)
		if numNg == nil {
			return num, nil
		}
	case int:
		return value, nil
	}
	fmt.Printf("toInt() argument: %v\n", any)
	return 0, errors.New("toInt() is failed.")
}

func toInterfaceSlice(any interface{}) []interface{} {
	switch typed := any.(type) {
	case []interface{}:
		return typed
	default:
		// if reflect.TypeOf(any).Kind() == reflect.Slice {
		// 	rv := reflect.MakeSlice(reflect.TypeOf(any), 0, 0)
		// 	rv = reflect.AppendSlice(rv, reflect.ValueOf(any))
		// 	result := []interface{}{}
		// 	for i := 0; i < rv.Len(); i++ {
		// 		result = append(result, rv.Index(i).Interface())
		// 	}
		// 	return result

		// } else {
		return []interface{}{any}
		// }
	}
}

func stripCallResult(callResult []reflect.Value) interface{} {
	//fmt.Printf("stripCallResult...%v\n", callResult)
	stripped := make([]interface{}, len(callResult))
	for index, value := range callResult {
		stripped[index] = value.Interface()
	}
	if len(stripped) == 1 {
		return stripped[0]
	} else {
		return stripped
	}
}

var calcPattern = regexp.MustCompile(`^([^\[\] ]+) +([-+*/%]) +([^\[\]]+)$`)
var comparePattern = regexp.MustCompile(`^([-$~\d][^ ]*?) *(<=|>=|<|>) *([-$~\d].*)$`)
var firstValuePattern = regexp.MustCompile(`^([^\[ \]\.]+)\.?(.+)$`)
var nextKeyPattern = regexp.MustCompile(`^(\[([^\[\]]+)\]|([^\[\] \.]+))\.?(.*)$`)
var dollerReplacePattern = regexp.MustCompile(`^(\$\.?)`)
var DslFunctions = map[string]func(map[string]interface{}, ...Argument) (interface{}, error){}
var DslAvailableFunctions = map[string]interface{}{}

func isFunc(any interface{}) bool {
	return reflect.ValueOf(any).Kind() != reflect.Invalid && strings.HasPrefix(reflect.TypeOf(any).String(), "func(")
}

func toReflectValues(array []interface{}) []reflect.Value {
	result := []reflect.Value{}
	for _, value := range array {
		result = append(result, reflect.ValueOf(value))
	}
	return result
}
func propertyGet(parent interface{}, key interface{}) (interface{}, error) {
	switch typedKey := key.(type) {
	case string:
		numKey, numOk := strconv.Atoi(typedKey)
		if numOk == nil {
			array := parent.([]interface{})
			return array[numKey], nil
		} else {
			switch typedParent := parent.(type) {
			case map[interface{}]interface{}:
				return typedParent[typedKey], nil
			case map[string]interface{}:
				return typedParent[typedKey], nil
			}
			tryValue := reflect.ValueOf(parent).MethodByName(key.(string))
			if tryValue.IsValid() {
				return tryValue.Interface(), nil
			}
			return nil, nil
		}
	case int:
		array := parent.([]interface{})
		return array[typedKey], nil
	}
	return nil, errors.New("propertyGet error: key type is invalid.")
}

func evaluateAll(args []Argument, container map[string]interface{}) ([]interface{}, error) {
	evaluated := make([]interface{}, len(args))
	for index, arg := range args {
		evaluatedValue, err := arg.Evaluate(container)
		if err == nil {
			evaluated[index] = evaluatedValue
		} else {
			return nil, err
		}
	}
	return evaluated, nil
}

func getLastKeyValue(container map[string]interface{}, arg Argument, root map[string]interface{}) ([]interface{}, error) {
	rawArg := arg.rawArg
	rootIsNil := root == nil
	if rootIsNil {
		root = container
	}
	switch rawArg.(type) {
	case string:
		rawArgStr := rawArg.(string)
		if rawArgStr == "$" {
			return []interface{}{"", root}, nil
		} else if val, ok := DslAvailableFunctions[rawArgStr]; ok {
			return []interface{}{"", val}, nil
		} else if !strings.Contains(rawArgStr, ".") && !strings.Contains(rawArgStr, "[") {
			return []interface{}{"", rawArgStr}, nil
		} else {
			var cursor interface{}
			cursor = container
			remainStr := rawArgStr
			if rootIsNil {
				firstValueMatch := firstValuePattern.FindStringSubmatch(remainStr)
				lastKeyValue, err := getLastKeyValue(container, Argument{firstValueMatch[1]}, nil)
				if err != nil {
					return nil, err
				}
				firstValue := lastKeyValue[1]
				if firstValue != nil {
					cursor = firstValue
					remainStr = firstValueMatch[2]
				} else {
					return []interface{}{nil, rawArgStr}, nil
				}
			}
			for {
				nextKeyMatch := nextKeyPattern.FindStringSubmatch(remainStr)
				if len(nextKeyMatch) != 0 {
					arrayKeyStr := nextKeyMatch[2]
					periodKeyStr := nextKeyMatch[3]
					remain := nextKeyMatch[4]
					var nextKey interface{}
					if periodKeyStr != "" {
						var nextKeyResult []interface{}
						var err error
						if arrayKeyStr != "" {
							nextKeyResult, err = getLastKeyValue(root, Argument{arrayKeyStr}, nil)
							if err != nil {
								return nil, err
							}
						} else {
							nextKeyResult, err = getLastKeyValue(root, Argument{periodKeyStr}, nil)
							if err != nil {
								return nil, err
							}
						}
						if nextKeyResult[0] == "" {
							nextKey = nextKeyResult[1]
						} else if nextKeyResult[0] == nil {
							nextKey = nil
						} else {
							result, _ := propertyGet(nextKeyResult[1], nextKeyResult[0])
							nextKey = result
						}
					} else {
						evaluated, err := Argument{arrayKeyStr}.Evaluate(container)
						if err == nil {
							nextKey = evaluated
						} else {
							return nil, err
						}
					}
					if remain == "" {
						return []interface{}{nextKey, cursor}, nil
					} else {
						result, err := propertyGet(cursor, nextKey)
						if err == nil {
							cursor = result
						} else {
							return nil, err
						}
						remainStr = remain
					}
				} else {
					return []interface{}{nil, nil}, nil
				}
			}
		}
	default:
		evaluated, err := arg.Evaluate(container)
		if err == nil {
			return []interface{}{"", evaluated}, nil
		} else {
			return nil, err
		}

	}
}

func getFirstKey(m map[interface{}]interface{}) string {
	var firstKey string
	for k, _ := range m {
		firstKey = k.(string)
		break
	}
	return firstKey
}

func asArray(any interface{}) []interface{} {
	result, ok := any.([]interface{})
	if ok {
		return result
	} else {
		return []interface{}{any}
	}
}

func (this Argument) Evaluate(container map[string]interface{}) (interface{}, error) {
	switch typedArg := this.rawArg.(type) {
	case string:
		if typedArg == "$" {
			return container, nil
		} else if comparePattern.MatchString(typedArg) {
			match := comparePattern.FindStringSubmatch(typedArg)
			return DslFunctions["compare"](
				container,
				NewArgument(match[2]),
				NewArgument(match[1]),
				NewArgument(match[3]))
		} else if calcPattern.MatchString(typedArg) {
			match := calcPattern.FindStringSubmatch(typedArg)
			var key string
			switch match[2] {
			case "+":
				key = "plus"
			case "-":
				key = "minus"
			case "*":
				key = "multiply"
			case "/":
				key = "divide"
			case "%":
				key = "mod"
			default:
			}
			if key != "" {
				return DslFunctions[key](container, NewArgument(match[1]), NewArgument(match[3]))
			}
		} else if strings.HasPrefix(typedArg, "$") {
			return DslFunctions["get"](container, NewArgument(typedArg))
		} else {
			_func, ok := DslAvailableFunctions[typedArg]
			if ok {
				return _func, nil
			}
		}
	case []interface{}:
		evaluated := make([]interface{}, len(typedArg))
		for index, arg := range typedArg {
			evaluatedValue, err := Argument{arg}.Evaluate(container)
			if err == nil {
				evaluated[index] = evaluatedValue
			} else {
				return nil, err
			}
		}
		return evaluated, nil
	case map[interface{}]interface{}:
		if len(typedArg) == 0 {
			return map[string]interface{}{}, nil
		} else if len(typedArg) == 1 {
			key := getFirstKey(typedArg)
			f, ok := DslFunctions[key]
			if ok {
				wrapped := []Argument{}
				for _, rawArg := range asArray(typedArg[key]) {
					wrapped = append(wrapped, NewArgument(rawArg))
				}
				result, err := f(container, wrapped...)
				return result, err // TBD
			} else if strings.HasPrefix(key, "$") {
				return DslFunctions["set"](container, NewArgument(key), Argument{typedArg[key]})
			}
		} else {
			result := map[string]interface{}{}
			for key, value := range typedArg {
				evaluated, err := NewArgument(value).Evaluate(container)
				if err != nil {
					return nil, err
				}
				result[key.(string)] = evaluated
			}
			return result, nil
		}
	default:
		//fmt.Println("what?", reflect.TypeOf(this.rawArg), this.rawArg)
	}
	return this.rawArg, nil
}

func init() {
	DslFunctions["print"] = func(container map[string]interface{}, args ...Argument) (interface{}, error) {
		evaluated, err := evaluateAll(args, container)
		if err == nil {
			fmt.Println(evaluated...)
			return nil, nil
		} else {
			return nil, err
		}
	}

	DslFunctions["set"] = func(container map[string]interface{}, args ...Argument) (interface{}, error) {
		evaluated, err := args[1].Evaluate(container)
		if err != nil {
			return nil, err
		}
		lastKeyValue, err := getLastKeyValue(container, args[0], nil)
		//fmt.Printf("set func lastKeyValue%v\n", lastKeyValue)
		//fmt.Println("set func lastKeyValue types...", reflect.TypeOf(lastKeyValue[0]), reflect.TypeOf(lastKeyValue[1]))
		if err != nil {
			return nil, err
		}
		key := lastKeyValue[0]
		parentValue := lastKeyValue[1]
		if parentValue != nil && key != nil && key != "" {
			switch typedKey := key.(type) {
			case string:
				numKey, numOk := strconv.Atoi(typedKey)
				if numOk == nil {
					parentValue.([]interface{})[numKey] = evaluated
				} else {
					parentValue.(map[string]interface{})[typedKey] = evaluated
					//fmt.Println("here?", parentValue)
				}
			case int:
				parentValue.([]interface{})[typedKey] = evaluated
			}
		}
		return nil, nil
	}
	DslFunctions["get"] = func(container map[string]interface{}, args ...Argument) (interface{}, error) {
		var firstArg Argument
		firstArg, args = args[0], args[1:]
		lastKeyValue, err := getLastKeyValue(container, firstArg, nil)
		//fmt.Printf("get func lastKeyValue%v\n", lastKeyValue)
		if err != nil {
			return nil, err
		}
		key := lastKeyValue[0]
		var defaultValue interface{}
		defaultValue = nil
		parentValue := lastKeyValue[1]
		if len(args) > 0 {
			_, ok := args[len(args)-1].rawArg.(string)
			if !ok {
				var lastArg Argument
				lastArg, args = args[len(args)-1], args[:len(args)-1]
				evaluated, err := lastArg.Evaluate(container)
				if err != nil {
					return nil, err
				}
				defaultValue = evaluated
			}
		}
		if parentValue != nil {
			if key == nil {
				return parentValue, nil
			} else {
				var cursor interface{}
				if key == "" {
					cursor = parentValue
				} else {
					switch typedKey := key.(type) {
					case string:
						numKey, numOk := strconv.Atoi(typedKey)
						if numOk == nil {
							cursor = parentValue.([]interface{})[numKey]
						} else {
							switch typedParentValue := parentValue.(type) {
							case map[string]interface{}:
								cursor = typedParentValue[typedKey]
							default:
								cursor = parentValue.(map[interface{}]interface{})[typedKey]
							}
						}
					case int:
						cursor = parentValue.([]interface{})[typedKey]
					}
				}
				for len(args) > 0 {
					var shiftArg Argument
					shiftArg, args = args[0], args[1:]
					key, err := shiftArg.Evaluate(container)
					if err != nil {
						return nil, err
					}
					switch typedCursor := cursor.(type) {
					case map[interface{}]interface{}:
						cursor = typedCursor[key.(string)]
					case map[string]interface{}:
						cursor = typedCursor[key.(string)]
					case []interface{}:
						cursor = typedCursor[key.(int)]
					}
				}
				if cursor == nil && len(args) == 0 {
					return defaultValue, nil
				}
				return cursor, nil
			}
		} else {
			return nil, nil
		}
		return nil, nil
	}
	DslFunctions["do"] = func(container map[string]interface{}, args ...Argument) (interface{}, error) {
		var firstArg Argument
		firstArg, args = args[0], args[1:]
		lastKeyValue, err := getLastKeyValue(container, firstArg, nil)
		//fmt.Printf("do func lastKeyValue%v\n", lastKeyValue)
		if err != nil {
			return nil, err
		}
		key := lastKeyValue[0]
		parentValue := lastKeyValue[1]
		if parentValue == nil || key == nil {
			return nil, nil
		}
		var cursor interface{}
		if key == "" {
			cursor = parentValue
		} else {
			result, _ := propertyGet(parentValue, key)
			cursor = result
		}
		for isFunc(cursor) == false && len(args) > 0 {
			var nextArg Argument
			nextArg, args = args[0], args[1:]
			key, err := nextArg.Evaluate(container)
			if err != nil {
				return nil, err
			}
			cursor, _ = propertyGet(cursor, key)
			if cursor == nil {
				break
			}
		}
		if isFunc(cursor) {
			evaluated, err := evaluateAll(args, container)
			if err != nil {
				return nil, err
			}
			reflectValues := toReflectValues(evaluated)
			callResult := reflect.ValueOf(cursor).Call(reflectValues)
			return stripCallResult(callResult), nil
		} else {
			return nil, nil
		}
	}

	DslFunctions["function"] = func(container map[string]interface{}, args ...Argument) (interface{}, error) {
		self := container
		fixedArguments := map[interface{}]interface{}{}
		argumentNames := args[0].rawArg
		process := args[1]
		if len(args) > 2 {
			for _, fixedKey := range asArray(args[2].rawArg) {
				evaluated, err := Argument{"$." + (fixedKey.(string))}.Evaluate(self)
				if err != nil {
					return nil, err
				}
				fixedArguments[fixedKey] = evaluated
			}
		}
		//return func(args ...interface{}) (interface{}, error) {
		return func(args ...interface{}) interface{} {
			for i, argumentName := range argumentNames.([]interface{}) {
				self[argumentName.(string)] = args[i]
			}
			self["this"] = container
			for k, v := range fixedArguments {
				self[k.(string)] = v
			}
			result, err := process.Evaluate(self)
			if err != nil {
				//return nil, err
				return err
			}
			delete(self, "exit")
			delete(self, "this")
			return result
		}, nil
	}
	DslFunctions["forEach"] = func(container map[string]interface{}, args ...Argument) (interface{}, error) {
		_self := container
		any, err := args[0].Evaluate(container)
		if err != nil {
			return nil, err
		}
		key := "item"
		if len(args) > 2 {
			key = args[2].rawArg.(string)
		}
		slice := toInterfaceSlice(any)
		for index, value := range slice {
			_self[key] = value
			_self["index"] = index
			args[1].Evaluate(_self)
		}
		return nil, nil
	}
	DslFunctions["filter"] = func(container map[string]interface{}, args ...Argument) (interface{}, error) {
		_self := container
		any, err := args[0].Evaluate(container)
		if err != nil {
			return nil, err
		}
		key := "item"
		if len(args) > 2 {
			key = args[2].rawArg.(string)
		}
		result := []interface{}{}
		slice := toInterfaceSlice(any)
		sliceSize := len(slice)
		for index, value := range slice {
			_self[key] = value
			_self["index"] = index
			evaluated, err := args[1].Evaluate(_self)
			if err != nil {
				return nil, err
			}
			if evaluated.(bool) {
				result = append(result, value)
			}
			if sliceSize-1 == index {
				delete(_self, key)
				delete(_self, "index")
			}
		}
		return result, nil
	}

	DslFunctions["map"] = func(container map[string]interface{}, args ...Argument) (interface{}, error) {
		_self := container
		any, err := args[0].Evaluate(container)
		if err != nil {
			return nil, err
		}
		key := "item"
		if len(args) > 2 {
			key = args[2].rawArg.(string)
		}
		result := []interface{}{}
		slice := toInterfaceSlice(any)
		sliceSize := len(slice)
		for index, value := range slice {
			_self[key] = value
			_self["index"] = index
			evaluated, err := args[1].Evaluate(_self)
			if err != nil {
				return nil, err
			}
			// if evaluated.(bool) {
			// 	result = append(result, value)
			// }
			//
			result = append(result, evaluated)
			//
			if sliceSize-1 == index {
				delete(_self, key)
				delete(_self, "index")
			}
		}
		return result, nil
	}

	DslFunctions["is"] = func(container map[string]interface{}, args ...Argument) (interface{}, error) {
		leftValueEvaluated, err := args[0].Evaluate(container)
		if err != nil {
			return nil, err
		}
		rightValueEvaluated, err := args[1].Evaluate(container)
		if err != nil {
			return nil, err
		}
		switch leftValue := leftValueEvaluated.(type) {
		case string:
			switch rightValue := rightValueEvaluated.(type) {
			case *regexp.Regexp:
				return rightValue.MatchString(leftValue), nil
			}
		case *regexp.Regexp:
			switch rightValue := rightValueEvaluated.(type) {
			case string:
				return leftValue.MatchString(rightValue), nil
			}

		}
		return leftValueEvaluated == rightValueEvaluated, nil
	}

	DslFunctions["not"] = func(container map[string]interface{}, args ...Argument) (interface{}, error) {
		result, err := DslFunctions["is"](container, args...)
		if err != nil {
			return nil, err
		} else {
			return !result.(bool), nil
		}
	}

	DslFunctions["format"] = func(container map[string]interface{}, args ...Argument) (interface{}, error) {
		formatString := args[0].rawArg.(string)
		args = args[1:]
		for _, arg := range args {
			evaluated, err := arg.Evaluate(container)
			if err != err {
				return nil, err
			}
			formatString = strings.Replace(formatString, "%s", toString(evaluated), 1)
		}
		return formatString, nil
	}

	DslFunctions["sequence"] = func(container map[string]interface{}, args ...Argument) (interface{}, error) {
		if _, ok := container["seqArray"]; !ok {
			container["seqArray"] = []interface{}{}
		}
		seqIndex := len(container["seqArray"].([]interface{}))
		for _, arg := range args {
			evaluated, err := arg.Evaluate(container)
			if err != err {
				return nil, err
			}
			if evaluated != nil {
				//fmt.Println("sequence 1", arg, evaluated)
				container["seq"] = evaluated
				if len(container["seqArray"].([]interface{})) == seqIndex {
					container["seqArray"] = append(container["seqArray"].([]interface{}), nil)
				}
				(container["seqArray"].([]interface{}))[seqIndex] = evaluated
			}
			if exit, _ := container["exit"]; exit == true {
				break
			}
		}
		container["seqArray"] = (container["seqArray"].([]interface{}))[0:seqIndex]
		return container["seq"], nil
	}

	DslFunctions["exit"] = func(container map[string]interface{}, args ...Argument) (interface{}, error) {
		container["exit"] = true
		return nil, nil
	}

	DslFunctions["plus"] = func(container map[string]interface{}, args ...Argument) (interface{}, error) {
		evaluated, err := evaluateAll(args, container)
		if err != err {
			return nil, err
		}
		result := 0
		for _, value := range evaluated {
			intValue, err := toInt(value)
			if err != err {
				return nil, err
			}
			result += intValue
		}
		return result, nil
	}

	DslFunctions["minus"] = func(container map[string]interface{}, args ...Argument) (interface{}, error) {
		evaluated, err := evaluateAll(args, container)
		if err != err {
			return nil, err
		}
		result, err := toInt(evaluated[0])
		if err != nil {
			panic(err)
		}
		evaluated = evaluated[1:]
		var intValue int
		for _, value := range evaluated {
			intValue, err = toInt(value)
			if err != nil {
				panic(err)
			}
			result -= intValue
		}
		return result, nil
	}

	DslFunctions["multiply"] = func(container map[string]interface{}, args ...Argument) (interface{}, error) {
		evaluated, err := evaluateAll(args, container)
		if err != err {
			return nil, err
		}
		result := 1
		for _, value := range evaluated {
			intValue, err := toInt(value)
			if err != err {
				return nil, err
			}
			result *= intValue
		}
		return result, nil
	}

	DslFunctions["divide"] = func(container map[string]interface{}, args ...Argument) (interface{}, error) {
		evaluated, err := evaluateAll(args, container)
		if err != err {
			return nil, err
		}
		result, err := toInt(evaluated[0])
		if err != err {
			return nil, err
		}
		evaluated = evaluated[1:]
		var intValue int
		for _, value := range evaluated {
			intValue, err = toInt(value)
			if err != err {
				return nil, err
			}
			result /= intValue
		}
		return result, nil
	}

	DslFunctions["mod"] = func(container map[string]interface{}, args ...Argument) (interface{}, error) {
		evaluated, err := evaluateAll(args, container)
		if err != err {
			return nil, err
		}
		result, err := toInt(evaluated[0])
		if err != err {
			return nil, err
		}
		evaluated = evaluated[1:]
		var intValue int
		for _, value := range evaluated {
			intValue, err = toInt(value)
			if err != err {
				return nil, err
			}
			result %= intValue
		}
		return result, nil
	}

	DslFunctions["compare"] = func(container map[string]interface{}, args ...Argument) (interface{}, error) {
		var leftIntValue, rightIntValue int
		leftEvaluated, err := args[1].Evaluate(container)
		if err != err {
			return nil, err
		}
		leftIntValue, err = toInt(leftEvaluated)
		if err != err {
			return nil, err
		}
		rightEvaluated, err := args[2].Evaluate(container)
		if err != err {
			return nil, err
		}
		rightIntValue, err = toInt(rightEvaluated)
		if err != nil {
			panic(err)
		}
		switch args[0].rawArg {
		case ">=":
			return leftIntValue >= rightIntValue, nil
		case "<=":
			return leftIntValue <= rightIntValue, nil
		case ">":
			return leftIntValue > rightIntValue, nil
		case "<":
			return leftIntValue < rightIntValue, nil
		}
		return nil, nil
	}

	DslFunctions["parseYaml"] = func(container map[string]interface{}, args ...Argument) (interface{}, error) {
		evaluated, err := args[0].Evaluate(container)
		if err != nil {
			return nil, err
		}
		var objInput map[interface{}]interface{}
		yamlError := yaml.UnmarshalStrict([]byte(evaluated.(string)), &objInput)
		if yamlError != nil {
			fmt.Println("unmarshal error:", err)
		}
		return objInput, nil
	}

	DslFunctions["now"] = func(container map[string]interface{}, args ...Argument) (interface{}, error) {
		return int(time.Now().UnixNano() / int64(time.Millisecond)), nil
	}

	DslFunctions["when"] = func(container map[string]interface{}, args ...Argument) (interface{}, error) {
		for len(args) > 0 {
			evaluated, err := args[0].Evaluate(container)
			if err != nil {
				return nil, err
			}
			if typedEvaluated, ok := evaluated.(bool); !ok {
				return nil, errors.New(fmt.Sprintf("%v: %v is not bool type.", args[0].rawArg, typedEvaluated))
			} else {
				if typedEvaluated {
					sequence, err := args[1].Evaluate(container)
					if err == nil {
						return sequence, nil
					} else {
						return nil, err
					}
				} else {
					args = args[2:]
				}
			}
		}
		return nil, errors.New(fmt.Sprintf("DslFunctions.when: no match (%v)", args))
	}

	DslFunctions["len"] = func(container map[string]interface{}, args ...Argument) (interface{}, error) {
		evaluated, err := args[0].Evaluate(container)
		if err != nil {
			return nil, err
		}
		return reflect.ValueOf(evaluated).Len(), nil
	}

	DslFunctions["reverse"] = func(container map[string]interface{}, args ...Argument) (interface{}, error) {
		evaluated, err := args[0].Evaluate(container)
		if err != nil {
			return nil, err
		}
		typedEvaluated, ok := evaluated.([]interface{})
		if !ok {
			return nil, errors.New(fmt.Sprintf("can't convert to []interface{}: %v", evaluated))
		}
		evaluatedLen := len(typedEvaluated)
		result := make([]interface{}, evaluatedLen)
		for index, value := range typedEvaluated {
			result[evaluatedLen-1-index] = value
		}
		return result, nil
	}

	DslFunctions["regexp"] = func(container map[string]interface{}, args ...Argument) (interface{}, error) {
		evaluated, err := args[0].Evaluate(container)
		if err != nil {
			return nil, err
		}
		typedEvaluated, ok := evaluated.(string)
		if !ok {
			return nil, errors.New(fmt.Sprintf("regexp 1st argument must be string. %v", evaluated))
		}
		compiled, err := regexp.Compile(typedEvaluated)
		if err != nil {
			return nil, err
		}
		return compiled, nil
	}

	DslFunctions["in"] = func(container map[string]interface{}, args ...Argument) (interface{}, error) {
		evaluated, err := args[0].Evaluate(container)
		if err != nil {
			return nil, err
		}
		groupEvaluated, err := args[1].Evaluate(container)
		if err != nil {
			return nil, err
		}
		if typedGroupEvaluated, ok := groupEvaluated.([]interface{}); ok {
			for _, groupValue := range typedGroupEvaluated {
				if regexpValue, ok := groupValue.(*regexp.Regexp); ok {
					contain := regexpValue.MatchString(toString(evaluated))
					if contain {
						return true, nil
					}
				} else {
					contain := evaluated == groupValue
					if contain {
						return true, nil
					}
				}
			}
		} else {
			fmt.Println("in: 2nd argument must be []interface{}.")
			return nil, errors.New("in: 2nd argument must be []interface{}.")
		}
		return false, nil
	}

	DslFunctions["testsuite"] = func(container map[string]interface{}, args ...Argument) (interface{}, error) {
		allCase := 0
		passedCase := 0
		failedCase := 0
		hasErrorCase := 0
		suiteName := args[0].rawArg
		container["suiteName"] = suiteName
		args = args[1:]
		result := []string{}
		for _, arg := range args {
			outputFlag := false
			evaluated, err := arg.Evaluate(container)
			if typedRawArg, ok := arg.rawArg.(map[interface{}]interface{}); ok {
				if _, ok := typedRawArg["testcase"]; ok {
					allCase++
					switch typedEvaluated := evaluated.(type) {
					case map[string]interface{}:
						if passed, ok := typedEvaluated["passed"]; ok {
							if typedPassed, ok := passed.(bool); ok && typedPassed {
								passedCase++
								//fmt.Println("passed", suiteName, allCase)
							} else {
								failedCase++
								outputFlag = true
								fmt.Println("failed", suiteName, allCase)
								result = append(result, fmt.Sprintf("case %v", allCase))
							}
						}
						if err != nil {
							hasErrorCase++
							outputFlag = true
							result = append(result, fmt.Sprintf("%v %v has error", suiteName, allCase))
						}
						if outputFlag {
							fmt.Println(suiteName, allCase)
							fmt.Println(evaluated)
						}
					}
				}

			}
		}
		if len(result) > 0 {
			return result, errors.New(fmt.Sprintf("testsuite: %v was failed.", suiteName))
		} else {
			return nil, nil
		}
	}

	DslFunctions["testcase"] = func(container map[string]interface{}, args ...Argument) (interface{}, error) {
		testResult := map[string]interface{}{
			"leftRaw":  args[0].rawArg,
			"rightRaw": args[1].rawArg,
			"passed":   false,
		}
		evaluated1, err := args[0].Evaluate(container)
		if err != nil {
			return testResult, err
		}
		testResult["leftEvaluated"] = evaluated1

		evaluated2, err := args[1].Evaluate(container)
		if err != nil {
			return testResult, err
		}
		testResult["rightEvaluated"] = evaluated2
		testResult["passed"] = evaluated1 == evaluated2
		return testResult, nil
	}

	DslFunctions["slice"] = func(container map[string]interface{}, args ...Argument) (interface{}, error) {
		slice, err := args[0].Evaluate(container)
		if err != nil {
			return nil, err
		}
		length, err := args[1].Evaluate(container)
		if err != nil {
			return nil, err
		}
		typedLength, ok := length.(int)
		if !ok {
			return nil, errors.New("slice 2nd argument must be int.")
		}
		if typedSlice, ok := slice.([]interface{}); ok {
			copied := make([]interface{}, typedLength)
			copy(copied, typedSlice)
			return copied, nil
		} else {
			return nil, errors.New("slice 1st argument must be []interface{}.")
		}
	}

	DslFunctions["and"] = func(container map[string]interface{}, args ...Argument) (interface{}, error) {
		if len(args) == 0 {
			return false, nil
		}

		for _, arg := range args {
			evaluated, err := arg.Evaluate(container)
			if err != nil {
				return nil, err
			}
			if typedEvaluated, ok := evaluated.(bool); ok {
				if !typedEvaluated {
					return false, nil
				}
			} else {
				fmt.Println(arg.rawArg)
				return nil, errors.New("and: evaluated is not bool")
			}
		}
		return true, nil
	}
}
