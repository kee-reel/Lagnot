package main

type TaskParamData struct {
	Name       string
	Type       string
	Range      []string
	IntRange   *[]int
	FloatRange *[]float64
	Dimensions *[]int
	TotalCount int
}

type Project struct {
	Id         int
	Name       string
	FolderName string
}

type Unit struct {
	Id         int
	NextId     *int
	Name       string
	FolderName string
}

type Task struct {
	Id         int
	Project    *Project
	Unit       *Unit
	Position   int
	FolderName string
	Extention  string
	Name       string
	Desc       string
	Input      []TaskParamData
	Output     string
	Path       string
	IsPassed   bool
}

type Token struct {
	Id     int
	UserId int
}

type Solution struct {
	Task                 *Task
	Source               string
	Path                 string
	ExecFilename         string
	CompleteExecFilename string
	TestCases            string
	Token                *Token
	IsVerbose            bool
}

type UserData struct {
	Token string
}

type ConfigDB struct {
	Host string
	Port string
	User string
	Pass string
	Name string
}

type ConfigServer struct {
	IsHTTP       bool
	EntryPoint   string
	Host         string
	Port         int
	CertFilename string
	KeyFilename  string
}

type Config struct {
	DB      ConfigDB
	Server  ConfigServer
	Verbose bool
}