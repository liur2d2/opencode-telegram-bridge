#!/bin/bash
set -e

echo "=========================================="
echo " OpenCode Telegram Bridge - 憑證設定"
echo "=========================================="
echo ""

# 檢查是否已有憑證
if [ -f ~/.opencode-telegram-credentials ]; then
    echo "發現現有憑證檔案，載入中..."
    source ~/.opencode-telegram-credentials
    echo "目前設定："
    echo "  Bot Token: ${TELEGRAM_BOT_TOKEN:0:10}..."
    echo "  Chat ID: $TELEGRAM_CHAT_ID"
    echo ""
    read -p "要使用現有憑證嗎？(y/n) " -n 1 -r
    echo ""
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        rm ~/.opencode-telegram-credentials
    else
        USE_EXISTING=true
    fi
fi

if [ -z "$USE_EXISTING" ]; then
    echo "請先完成以下步驟："
    echo ""
    echo "1. 建立 Telegram Bot (如果還沒建立)："
    echo "   - 在 Telegram 搜尋 @BotFather"
    echo "   - 傳送: /newbot"
    echo "   - 依指示設定 bot 名稱"
    echo "   - 複製 BotFather 給你的 token"
    echo ""
    echo "2. 取得你的 Chat ID："
    echo "   - 在 Telegram 搜尋 @userinfobot"
    echo "   - 傳送任意訊息"
    echo "   - 複製回傳的 ID 數字"
    echo ""
    echo "=========================================="
    echo ""

    # 輸入 Bot Token
    read -p "請輸入你的 Telegram Bot Token: " BOT_TOKEN
    if [ -z "$BOT_TOKEN" ]; then
        echo "錯誤: Bot Token 不能為空"
        exit 1
    fi

    # 輸入 Chat ID
    read -p "請輸入你的 Chat ID: " CHAT_ID
    if [ -z "$CHAT_ID" ]; then
        echo "錯誤: Chat ID 不能為空"
        exit 1
    fi

    # 驗證 Chat ID 是數字
    if ! [[ "$CHAT_ID" =~ ^-?[0-9]+$ ]]; then
        echo "錯誤: Chat ID 必須是數字"
        exit 1
    fi

    # 儲存憑證
    cat > ~/.opencode-telegram-credentials << EOF
export TELEGRAM_BOT_TOKEN="$BOT_TOKEN"
export TELEGRAM_CHAT_ID="$CHAT_ID"
EOF
    chmod 600 ~/.opencode-telegram-credentials

    TELEGRAM_BOT_TOKEN="$BOT_TOKEN"
    TELEGRAM_CHAT_ID="$CHAT_ID"

    echo ""
    echo "✅ 憑證已儲存到 ~/.opencode-telegram-credentials"
fi

echo ""
echo "=========================================="
echo " 設定 LaunchAgent"
echo "=========================================="
echo ""

# 取得當前使用者
USERNAME=$(whoami)
PROJECT_DIR="/Users/$USERNAME/opencode-telegram"
CONFIG_DIR="/Users/$USERNAME/.config/opencode"

# 更新 plist 檔案
PLIST_PATH="$PROJECT_DIR/configs/com.opencode.telegram-bridge.plist"

echo "正在設定 plist 檔案..."

# 使用 sed 替換 placeholder
sed -e "s|YOUR_USERNAME|$USERNAME|g" \
    -e "s|YOUR_BOT_TOKEN_HERE|$TELEGRAM_BOT_TOKEN|g" \
    -e "s|YOUR_CHAT_ID_HERE|$TELEGRAM_CHAT_ID|g" \
    "$PLIST_PATH" > "$PLIST_PATH.tmp"

mv "$PLIST_PATH.tmp" "$PLIST_PATH"

echo "✅ Plist 設定完成"
echo ""
echo "接下來執行："
echo "  ./scripts/install.sh"
echo ""
echo "或是手動測試："
echo "  cd $PROJECT_DIR"
echo '  export TELEGRAM_BOT_TOKEN="'"$TELEGRAM_BOT_TOKEN"'"'
echo "  export TELEGRAM_CHAT_ID=$TELEGRAM_CHAT_ID"
echo "  ./opencode-telegram"
echo ""
