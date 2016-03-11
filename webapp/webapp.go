package webapp

import (
  "github.com/wpxiong/beargo/log"
  "github.com/wpxiong/beargo/process"
  "github.com/wpxiong/beargo/route"
  "github.com/wpxiong/beargo/appcontext"
  "github.com/wpxiong/beargo/webhttp"
  "github.com/wpxiong/beargo/controller"
  "github.com/wpxiong/beargo/util"
  "github.com/wpxiong/beargo/filter"
  "github.com/wpxiong/beargo/render"
  "github.com/wpxiong/beargo/constvalue"
  "github.com/wpxiong/beargo/memorycash"
  "github.com/wpxiong/beargo/render/template"
  "github.com/wpxiong/beargo/session"
  "github.com/wpxiong/beargo/session/provider"
  "strconv"
  "time"
  "os"
  "net/http"
  "reflect"
  "strings"
)


var Default_TimeOut int = 180
 
func init() {
  log.InitLog()
}

type FilterType int 

const (
    BeforeFilter   FilterType = iota
    AfterFilter 
)

type  WebApplication struct {
   WorkProcess *process.WorkProcess
   RouteProcess  *route.RouteProcess
   AppContext *appcontext.AppContext
   IsStart bool
   control  chan int
   resourceUrlPath  string
}

type ConfigMap struct {
  Filterfuncmap    map[string]filter.FilterFunc
  Sessionprovidermap  map[string]session.SessionProvider 
  Templatefuncmap  template.TemplateFuncMap
}


var webApp *WebApplication

func InitDefaultConvertFunction(appContext *appcontext.AppContext){
  appContext.InitAppContext(appContext.ConfigContext.ConfigPath ,appContext.ConfigContext.Port)
  appContext.AddConvertFunctiont("Int",util.StringToInt)
  appContext.AddConvertFunctiont("Float",util.StringToFloat)
  appContext.AddConvertFunctiont("Double",util.StringToDouble)
  appContext.AddConvertFunctiont("Bool",util.StringToBool)
}

func MergeMapFilterFuncMap(dest  map[string]filter.FilterFunc, src  map[string]filter.FilterFunc) {
  for key,val := range src {
     if dest[key] == nil {
        dest[key] = val
     }
  }
}


func MergeMapSessionProviderMap(dest  map[string]session.SessionProvider , src  map[string]session.SessionProvider ) {
  for key,val := range src {
     if dest[key] == nil {
        dest[key] = val
     }
  }
}

func MergeMapTemplateFuncMap(dest  template.TemplateFuncMap , src template.TemplateFuncMap ) {
  for key,val := range src {
     if dest[key] == nil {
        dest[key] = val
     }
  }
}

func initDefaultSessionProviderMap() map[string]session.SessionProvider {
  sessionProviderMap := make(map[string]session.SessionProvider)
  sessionProviderMap[constvalue.DEFAULT_SESSION_PROVIDER] = &provider.MemorySessionProvider{}
  return sessionProviderMap
}


func initDefaultTemplateFuncMap() template.TemplateFuncMap {
  funcMap := make(template.TemplateFuncMap)
  return funcMap
}


func initDefaultFilterFuncMap() map[string]filter.FilterFunc {
  funcMap := make(map[string]filter.FilterFunc)
  funcMap[constvalue.ParameterParseFilter] = filter.ParameterParseFilter
  funcMap[constvalue.ParameterBinderFilter] =  filter.ParameterBinderFilter
  funcMap[constvalue.RenderBindFilter] =  filter.RenderBindFilter
  funcMap[constvalue.RenderOutPutFilter] =  filter.RenderOutPutFilter
  return funcMap
}

func New(appContext *appcontext.AppContext, configMap ConfigMap) *WebApplication {
   if webApp == nil {
      webApp = &WebApplication{WorkProcess : process.New(),RouteProcess : route.NewRouteProcess(appContext) , AppContext : appContext , control:make(chan int ) }
      InitDefaultConvertFunction(appContext)
      filter.InitFilter()
      
      memorycash.CreateMemoryCashManager(appContext)
          
      //filterFuncMap
      default_funcmap := initDefaultFilterFuncMap()
      MergeMapFilterFuncMap(default_funcmap,configMap.Filterfuncmap)
      filter.AddInitFilter(appContext,default_funcmap)
      
      //SessionProviderMap
      default_session_provider := initDefaultSessionProviderMap()
      MergeMapSessionProviderMap(default_session_provider,configMap.Sessionprovidermap)
      session.CreateSessionManager(appContext,default_session_provider)
      
      pwd, _ := os.Getwd()
      webApp.resourceUrlPath = appContext.GetConfigValue(constvalue.RESOURCE_PATH_KEY,constvalue.DEFAULT_RESOURCE_PATH).(string)
      render.SetDefaultTemplateDir(pwd)
      
      //Templatefuncmap
      default_template_func := initDefaultTemplateFuncMap()
      MergeMapTemplateFuncMap(default_template_func,configMap.Templatefuncmap)
      render.CreateSessionManager(appContext,default_template_func)
   }
   return webApp;
}

func processWebRequest(param interface{}) interface{} {
   var rti *route.RouteInfo = param.(*route.RouteInfo)
   rti.CallMethod()
   rti.ResultChan <- 1
   return true
}

func processRequest(w http.ResponseWriter, r *http.Request){
    path := r.URL.Path
    if strings.HasPrefix(path,webApp.resourceUrlPath) {
       filePath := constvalue.RESOURCE_FOLDER + path[len(webApp.resourceUrlPath):]
       http.ServeFile(w, r,filePath)
       return 
    }
    log.Info("Request Url: " + path )
    request := webhttp.HttpRequest{Urlpath : path, HttpRequest : r }
    response := webhttp.HttpResponse{HttpResponseWriter:&w}
    var rti *route.RouteInfo
    rti = webApp.RouteProcess.ProcessRequest(&request)
    if rti.GetResult() {
       rti.Request = &request
       rti.Writer =&response
       workjob := &process.WorkJob{Parameter : rti }
       workjob.WorkProcess = processWebRequest
       process.AddJob(webApp.WorkProcess,workjob)
       _ = <- rti.ResultChan
        
    }else {
       log.Error("Error: not found page")
    }
}

func startProcess(web *WebApplication){
    requestTimeout := web.AppContext.GetConfigValue(constvalue.REQUEST_TIMEOUT_KEY,constvalue.DEFAULT_REQUEST_TIMEOUT).(string)
    var resqTimeout,respTimeout int
    var err error
    resqTimeout,err =  strconv.Atoi(requestTimeout)
    if err == nil {
       resqTimeout = Default_TimeOut
    }
    responseTimeout := web.AppContext.GetConfigValue(constvalue.RESPONSE_TIMEOUT_KEY,constvalue.DEFAULT_RESPONSE_TIMEOUT).(string)
    respTimeout,err =  strconv.Atoi(responseTimeout)
    if err == nil {
       respTimeout = Default_TimeOut
    }
    strconv.Atoi(responseTimeout)
    server := &http.Server{
	   Addr:           ":" + strconv.Itoa(web.AppContext.ConfigContext.Port),
	   Handler:        http.HandlerFunc(processRequest),
	   ReadTimeout:    time.Duration(resqTimeout * int(time.Second)),
	   WriteTimeout:   time.Duration(respTimeout * int(time.Second)),
	   MaxHeaderBytes: 1 << 20,
	}
    err = server.ListenAndServe()
    if err != nil {
        log.DebugNoReturn("ListenAndServe: ")
        log.ErrorArray("Error",err)
        web.control <- 1
    }
}

func (web *WebApplication) AddAutoRoute(urlPattern string,controller controller.ControllerMethod,form interface{}) {
   var formType reflect.Type = reflect.TypeOf(form)
   web.RouteProcess.AddAuto(urlPattern,controller,formType)
}

func (web *WebApplication) AddAutoRouteWithViewPath(urlPattern string,controller controller.ControllerMethod,form interface{},viewPath string) {
   var formType reflect.Type = reflect.TypeOf(form)
   web.RouteProcess.AddAutoWithViewPath(urlPattern,controller,formType,viewPath)
}

func (web *WebApplication) AddRouteWithViewPath(urlPattern string,controller controller.ControllerMethod,method string,form interface{},viewPath string) {
   var formType reflect.Type = reflect.TypeOf(form)
   web.RouteProcess.AddWithViewPath(urlPattern,controller,method,formType,viewPath)
}

func (web *WebApplication) AddRoute(urlPattern string,controller controller.ControllerMethod,method string,form interface{}) {
   var formType reflect.Type = reflect.TypeOf(form)
   web.RouteProcess.Add(urlPattern,controller,method,formType)
}

func (web *WebApplication) AddFilter(filterfunc filter.FilterFunc,filterType FilterType) {
   switch filterType {
     case BeforeFilter:
        filter.AddBeforeFilter(filterfunc)
     case AfterFilter:
        filter.AddAfterFilter(filterfunc)
   }
}

func (web *WebApplication) SetTemplateWorkDictionary(folerpath string){
   render.SetTemplateDir(folerpath)
}

func (web *WebApplication) Start() {
    go startProcess(web)
    if err := render.CompileTemplate();err != nil {
       log.Error(err)
       return
    }
    web.WorkProcess.Init_Default()
    res := <- web.control
    if res == 1 {
       process.StopWork(webApp.WorkProcess)
       log.Info("Stop WebApplication")
    }
}