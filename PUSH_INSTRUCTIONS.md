# 如何推送到 GitHub

## 當前狀態

✅ 所有變更已提交
✅ Git remote 已設定為: `https://github.com/liur2d2/opencode-telegram-bridge.git`
✅ 繁體中文和英文 README 已創建

## 最後一步：推送到 GitHub

由於需要 GitHub 認證，請選擇以下任一方式：

### 方式 1: 使用 GitHub CLI（推薦）

```bash
cd /Users/master/opencode-telegram
gh auth login
# 選擇：GitHub.com → HTTPS → Login with a web browser
# 完成後執行：
git push -u origin main
```

### 方式 2: 使用 Personal Access Token

1. 在瀏覽器前往：https://github.com/settings/tokens/new
2. 創建 token，勾選 `repo` 權限
3. 複製 token，然後執行：

```bash
cd /Users/master/opencode-telegram
git push -u origin main
# Username: liur2d2
# Password: [貼上你的 token]
```

### 方式 3: 設定 SSH Key

```bash
# 生成 SSH key
ssh-keygen -t ed25519 -C "your_email@example.com"
# 添加到 ssh-agent
eval "$(ssh-agent -s)"
ssh-add ~/.ssh/id_ed25519
# 複製公鑰
cat ~/.ssh/id_ed25519.pub
# 將公鑰添加到 GitHub: https://github.com/settings/keys
# 切換到 SSH URL
git remote set-url origin git@github.com:liur2d2/opencode-telegram-bridge.git
# 推送
git push -u origin main
```

## 驗證

推送成功後，檢查：https://github.com/liur2d2/opencode-telegram-bridge

應該看到：
- ✅ README.md（英文版）
- ✅ README.zh-TW.md（繁體中文版）
- ✅ 所有專案文件
