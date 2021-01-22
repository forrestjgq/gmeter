package meter

var gVars = make(map[string]string)

func AddGlobalVariable(k, v string) {
	if len(k) > 0 && len(v) > 0 {
		gVars[k] = v
	}
}
func GetGlobalVariable(k string) string {
	return gVars[k]
}
