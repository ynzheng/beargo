package interceptor

import (
  "github.com/wpxiong/beargo/log"
  "github.com/wpxiong/beargo/appcontext"
  "github.com/wpxiong/beargo/session"
  "github.com/wpxiong/beargo/constvalue"
)

func init() {
  log.InitLog()
}

func Xsrfinterceptor(app *appcontext.AppContext) bool {
   log.Debug("Xsrfinterceptor Start")
   var sess session.Session  = session.NewSession(app.Request.HttpRequest , *app.Writer.HttpResponseWriter)
   var sessionValue string
   if sess.GetSessionValue(constvalue.XSRF_TOKEN,&sessionValue)  {
      var token string 
      if app.Parameter[constvalue.XSRF_TOKEN] == nil {
        return false
      }else {
        token = app.Parameter[constvalue.XSRF_TOKEN].(string)
      }
      if token != sessionValue {
         return false
      }
   }
   return true
}