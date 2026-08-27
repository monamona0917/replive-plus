# Replive+

基于 [Chilfish/replive-oyu](https://github.com/Chilfish/replive-oyu) 和 [huangwg2529/nsy_chat_live](https://github.com/huangwg2529/nsy_chat_live) 进行重构和扩展

详细使用说明参照原项目nsy_chat_live中提供的 https://my.feishu.cn/wiki/PXe9wkiksifZR9kpVoucsKs1nQe

### 具体修改内容

- 重构Web 前端，可在浏览器中使用聊天记录。
- 支持显示Fandom已读状态。
- 显示 Fandom 订阅天数及对应阶段样式。显示账号可输入字符上限。
- 新增 Prime Chat 房间

## 构建与运行（Windows）

需要预先安装 Go 1.24+ 与 Node.js 20+。双击根目录的 `build.bat`，首次运行会安装前端依赖，并在 `dist` 生成：

- `replive-plus.exe`：后端
- `replive-plus-web.exe`：内置 Web 前端

