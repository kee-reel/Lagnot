package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"late/models"
	"late/storage"
	"late/utils"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var user_tests_re = regexp.MustCompile(`^((-?\d+;)+\n)+$`)

type APISolutionVerboseResult struct {
	Params string `example:"2;1;7;'"`
	Result string `example:"8"`
}
type APISolutionErrorData struct {
	Stage    string `example:"-"`
	Msg      string `example:"Build fail message"`
	Params   string `example:"2;1;7'"`
	Expected string `example:"8"`
	Result   string `example:"1"`
}

type APITestResult struct {
	Error     WebError                    `json:"error,omitempty" example:"508"`
	ErrorData *APISolutionErrorData       `json:"error_data,omitempty"`
	Result    *[]APISolutionVerboseResult `json:"result,omitempty"`
}

// @Tags solution
// @Summary Send solution for specific task
// @Description Receives solution in form of file or plain text.
// @Description Builds solution and then runs. While running it gives various input parameters (through stdin) and expects specific result (from stdout).
// @Description Apart from errors raised due to invalid POST parameters, there are 2 "normal" errors:
// @Description 508 - Solution build error. If this happens, then result will contain: `{"error":508,"error_data":{"msg":"multiline compilation error"}}`
// @Description 509 - Solution test error. If this happens, then result will contain: `{"error":509,"error_data":{"expected":"expected result", "params":"semicolon separated input parameters", "result":"actual result"}}`
// @Description If "verbose" flag is "true" then result will contain (if no error occurs): `{"result":[{"params":"semicolon separated input parameters", "result":"actual result"}]}`
// @ID post-solution
// @Produce  json
// @Param   token   query    string  true    "Access token returned by GET /login"
// @Param   lang   formData    string  true    "Language of passing solution"
// @Param   task_id   formData    int  true    "ID of task to pass with given solution"
// @Param   source_text   formData    string  false    "Source text of passing solution"
// @Param   source_file   formData    file  false    "File with source text of passing solution"
// @Param   test_cases   formData    string  false    "User test cases for solution"
// @Param   verbose   formData    bool  false    "If specified - when solution is passed, all test results will be returned"
// @Success 200 {object} main.APISolutionTestResult "Success"
// @Failure 400 {object} main.APISolutionTestResult "Possible error codes: 300, 301, 302, 304, 4XX, 5XX, 6XX"
// @Failure 500 {object} main.APIInternalError "Server internal bug"
// @Router /solution [post]
func PostSolution(r *http.Request) (interface{}, WebError) {
	solution, web_err := ParseSolution(r)
	if web_err != NoError {
		return nil, web_err
	}
	test_result := BuildAndTest(solution.Task, solution)
	storage.SaveSolution(solution, test_result.Error == NoError)
	return test_result, test_result.Error
}

func ParseSolution(r *http.Request) (*models.Solution, WebError) {
	err := r.ParseMultipartForm(32 << 20)
	token_str, web_err := getUrlParam(r, "token")
	if web_err != NoError {
		return nil, web_err
	}
	token, web_err := getToken(r, token_str)
	if web_err != NoError {
		return nil, web_err
	}
	lang, web_err := getFormParam(r, "lang")
	if web_err != NoError {
		return nil, web_err
	}
	task_id_str, web_err := getFormParam(r, "task_id")
	if web_err != NoError {
		return nil, web_err
	}
	task_id, err := strconv.Atoi(*task_id_str)
	if err != nil {
		return nil, TaskIdInvalid
	}

	task_ids := []int{task_id}
	tasks := storage.GetTasks(token, task_ids)
	if len(*tasks) == 0 {
		return nil, TaskNotFound
	}

	var solution models.Solution
	task := (*tasks)[0]

	var solution_text *string
	source_text := r.FormValue("source_text")
	if source_text != "" {
		solution_text = &source_text
	} else {
		file, _, err := r.FormFile("source_file")
		if err != nil {
			return nil, SolutionTextNotProvided
		}
		raw_data, err := ioutil.ReadAll(file)
		utils.Err(err)
		str_data := string(raw_data)
		solution_text = &str_data
	}

	if len(*solution_text) > 50000 {
		return nil, SolutionTextTooLong
	}

	solution.Source = *solution_text
	solution.TestCases = r.FormValue("test_cases")
	if len(solution.TestCases) > 50000 {
		return nil, SolutionTestsTooLong
	}
	if len(solution.TestCases) > 0 {
		solution.TestCases = strings.Replace(solution.TestCases, "\r", "", -1)
		matches := user_tests_re.MatchString(solution.TestCases)
		if !matches {
			return nil, SolutionTestsInvalid
		}
	}

	solution.Task = &task
	solution.Token = token
	solution.IsVerbose = r.FormValue("verbose") == "true"
	solution.Extention = *lang

	return &solution, NoError
}

func BuildAndTest(task *models.Task, solution *models.Solution) *APITestResult {
	complete_solution_source, fixed_tests := storage.GetTaskTestData(task.Id)
	random_tests := GenerateTests(task)

	runner_url := fmt.Sprintf("http://%s:%s", utils.Env("RUNNER_HOST"), utils.Env("RUNNER_PORT"))
	verbose_text := "false"
	if utils.EnvB("RUNNER_VERBOSE") && solution.IsVerbose {
		verbose_text = "true"
	}
	response, err := http.PostForm(runner_url, url.Values{
		"solution":              {solution.Source},
		"complete_solution":     {*complete_solution_source},
		"user_tests":            {solution.TestCases},
		"fixed_tests":           {*fixed_tests},
		"random_tests":          {*random_tests},
		"solution_ext":          {solution.Extention},
		"complete_solution_ext": {task.Extention},
		"verbose":               {verbose_text},
	})
	utils.Err(err)
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	utils.Err(err)

	var test_result APITestResult
	err = json.Unmarshal([]byte(body), &test_result)
	utils.Err(err)

	if test_result.ErrorData != nil {
		switch string(test_result.ErrorData.Stage) {
		case "build":
			test_result.Error = SolutionBuildFail
		case "test":
			test_result.Error = SolutionTestFail
		}
	}
	return &test_result
}

func GenerateTests(task *models.Task) *string {
	result := ""
	if len(task.Input) == 0 {
		return &result
	}
	random_tests_count := 10
	test_case_size := 1 // To add '\n' after every test case
	for _, input := range task.Input {
		test_case_size += input.TotalCount
		if input.TotalCount > 1 {
			for _, d := range input.Dimensions {
				if d != 0 {
					test_case_size++
				}
			}
		}
	}
	test_data := make([]string, random_tests_count*test_case_size)
	rand.Seed(time.Now().UnixNano())
	start_index := 0
	for i := 0; i < random_tests_count; i++ {
		for _, input := range task.Input {
			start_index = GenTestParam(test_data, input, start_index)
		}
		test_data[start_index] = "\n"
		start_index++
	}
	result = strings.Join(test_data, "")
	return &result
}

func GenTestParam(test_data []string, param models.TaskParamData, start_index int) int {
	delimiter := ';'
	values_to_generate := 1
	cur_d := 0
	if param.TotalCount > 1 {
		for _, d := range param.Dimensions {
			if d != 0 {
				cur_d = 1 + rand.Intn(d)
				if cur_d == 1 {
					cur_d = 1 + rand.Intn(d)
				}
				test_data[start_index] = fmt.Sprintf("%d%c", cur_d, delimiter)
				start_index++
			}
			values_to_generate *= cur_d
		}
	}
	last_index := start_index + values_to_generate
	switch param.Type {
	case "float", "double":
		min := (*param.FloatRange)[0]
		value_range := math.Abs((*param.FloatRange)[1]-min) + 1
		type_spec := "%f%c"
		for i := start_index; i < last_index; i++ {
			value := min + rand.Float64()*value_range
			test_data[i] = fmt.Sprintf(type_spec, value, delimiter)
		}
	case "int":
		min := (*param.IntRange)[0]
		value_range := utils.Abs((*param.IntRange)[1]-min) + 1
		type_spec := "%d%c"
		for i := start_index; i < last_index; i++ {
			value := min + rand.Intn(value_range)
			test_data[i] = fmt.Sprintf(type_spec, value, delimiter)
		}
	default:
		utils.Err(fmt.Errorf("Unknown parameter type: %s", param.Type))
	}
	return last_index
}