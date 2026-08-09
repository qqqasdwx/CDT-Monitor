# CDT Monitor Frontend

React + Vite + TypeScript 前端，使用 Tailwind CSS 和项目内 shadcn/ui 风格基础组件。

## 页面入口

本前端使用 Vite 多 HTML 入口，不使用单一 React Router 入口承载所有页面：

- `login.html`：登录页
- `dashboard.html`：运行控制台
- `settings.html`：配置管理

`index.html` 只负责跳转到控制台入口。

## 本地开发

```bash
npm install
npm run dev
```

开发服务器通过 Vite proxy 转发 `/api` 和 `/healthz` 到 `http://localhost:8080`。后端必须设置 `CDTM_ADMIN_PASSWORD` 后启动。

## 验证

```bash
npm run lint
npm run build
```
