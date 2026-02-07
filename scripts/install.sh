#!/bin/bash
set -e

PLIST_SRC="configs/com.opencode.telegram-bridge.plist"
PLIST_DEST="$HOME/Library/LaunchAgents/com.opencode.telegram-bridge.plist"

echo "🚀 Installing opencode-telegram LaunchAgent..."

# Create log directory
mkdir -p "$HOME/Library/Logs/opencode-telegram"

# Copy plist
cp "$PLIST_SRC" "$PLIST_DEST"
echo "📋 Plist installed at: $PLIST_DEST"

# Load service
launchctl load "$PLIST_DEST"
echo "✅ Service installed and started"
echo ""
echo "📝 Next steps:"
echo "1. Edit the plist to add your credentials:"
echo "   nano $PLIST_DEST"
echo ""
echo "2. Required environment variables:"
echo "   - TELEGRAM_BOT_TOKEN: Your Telegram bot token from @BotFather"
echo "   - TELEGRAM_CHAT_ID: Your chat ID (get via /chatid command)"
echo "   - OPENCODE_BASE_URL: Default is http://localhost:54321"
echo "   - OPENCODE_DIRECTORY: Path to your OpenCode config (default ~/.config/opencode)"
echo ""
echo "3. Verify service is running:"
echo "   launchctl list | grep com.opencode.telegram-bridge"
echo ""
echo "4. Check logs:"
echo "   tail -f $HOME/Library/Logs/opencode-telegram/stdout.log"
echo "   tail -f $HOME/Library/Logs/opencode-telegram/stderr.log"
echo ""
echo "5. To unload service:"
echo "   launchctl unload $PLIST_DEST"
