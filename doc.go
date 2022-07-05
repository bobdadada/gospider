// go爬虫基础框架，包括代理池和Cookies池
//
// - 代理池：
//    1. 获取模块
//    2. 存储模块
//    3. 接口模块
//    4. 检测模块
//
// - Cookie池：
//    Cookie池保存了许多Cookie信息，并且定时检测每个Cookie的有效性，如果
// 某Cookie无效，那就删除该Cookie并模拟登录生成新的Cookie。同时Cookie池还
// 需要一个非常重要的接口，即获取随机Cookie的接口，Cookie池运行后，我们只需
// 要请求该接口。
//    1. 生成模块
//    2. 存储模块
//    3. 接口模块
//    4. 检测模块
//

package gospider
