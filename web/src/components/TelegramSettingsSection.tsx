import { useState, useEffect } from 'react'
import { Send, Bell, AlertCircle, CheckCircle, Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '../lib/api'
import type { TelegramSettings } from '../types'

interface TelegramSettingsSectionProps {
  traderId: string
  traderName?: string
}

export function TelegramSettingsSection({ traderId, traderName }: TelegramSettingsSectionProps) {
  const [settings, setSettings] = useState<TelegramSettings>({
    trader_id: traderId,
    chat_id: 0,
    enabled: false,
    notify_open: true,
    notify_close: true,
    notify_errors: false,
  })
  const [isLoading, setIsLoading] = useState(true)
  const [isSaving, setIsSaving] = useState(false)
  const [isTesting, setIsTesting] = useState(false)

  useEffect(() => {
    const fetchSettings = async () => {
      try {
        const data = await api.getTelegramSettings(traderId)
        setSettings(data)
      } catch {
        // Use default settings if not found
      } finally {
        setIsLoading(false)
      }
    }
    if (traderId) {
      fetchSettings()
    }
  }, [traderId])

  const handleSave = async () => {
    setIsSaving(true)
    try {
      const updated = await api.updateTelegramSettings(traderId, settings)
      setSettings(updated)
      toast.success('Telegram 设置已保存')
    } catch (err) {
      toast.error('保存失败')
    } finally {
      setIsSaving(false)
    }
  }

  const handleTest = async () => {
    if (!settings.chat_id) {
      toast.error('请先配置 Chat ID')
      return
    }
    setIsTesting(true)
    try {
      await api.testTelegramNotification(traderId, `测试通知 - 交易员: ${traderName || traderId}`)
      toast.success('测试通知已发送')
    } catch {
      toast.error('发送失败，请检查 Bot Token 和 Chat ID 配置')
    } finally {
      setIsTesting(false)
    }
  }

  const handleChange = <K extends keyof TelegramSettings>(key: K, value: TelegramSettings[K]) => {
    setSettings(prev => ({ ...prev, [key]: value }))
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-8">
        <Loader2 className="w-5 h-5 animate-spin text-[#848E9C]" />
      </div>
    )
  }

  return (
    <div className="space-y-4">
      {/* Enable Toggle */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Bell className="w-4 h-4 text-[#F0B90B]" />
          <span className="text-sm text-[#EAECEF]">启用 Telegram 通知</span>
        </div>
        <button
          type="button"
          onClick={() => handleChange('enabled', !settings.enabled)}
          className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
            settings.enabled ? 'bg-[#F0B90B]' : 'bg-[#2B3139]'
          }`}
        >
          <span
            className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
              settings.enabled ? 'translate-x-6' : 'translate-x-1'
            }`}
          />
        </button>
      </div>

      {settings.enabled && (
        <>
          {/* Chat ID Input */}
          <div>
            <label className="text-sm text-[#EAECEF] block mb-2">
              Chat ID <span className="text-red-500">*</span>
            </label>
            <input
              type="number"
              value={settings.chat_id || ''}
              onChange={(e) => handleChange('chat_id', Number(e.target.value))}
              className="w-full px-3 py-2 bg-[#0B0E11] border border-[#2B3139] rounded text-[#EAECEF] focus:border-[#F0B90B] focus:outline-none"
              placeholder="输入您的 Telegram Chat ID"
            />
            <p className="text-xs text-[#848E9C] mt-1">
              发送 /start 给 @nofx_trade_bot 获取您的 Chat ID
            </p>
          </div>

          {/* Notification Types */}
          <div className="space-y-3">
            <label className="text-sm text-[#EAECEF] block">通知类型</label>

            <div className="flex items-center justify-between p-3 bg-[#1E2329] rounded">
              <div className="flex items-center gap-2">
                <CheckCircle className="w-4 h-4 text-green-400" />
                <span className="text-sm text-[#EAECEF]">开仓通知</span>
              </div>
              <button
                type="button"
                onClick={() => handleChange('notify_open', !settings.notify_open)}
                className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors ${
                  settings.notify_open ? 'bg-[#F0B90B]' : 'bg-[#2B3139]'
                }`}
              >
                <span
                  className={`inline-block h-3 w-3 transform rounded-full bg-white transition-transform ${
                    settings.notify_open ? 'translate-x-5' : 'translate-x-1'
                  }`}
                />
              </button>
            </div>

            <div className="flex items-center justify-between p-3 bg-[#1E2329] rounded">
              <div className="flex items-center gap-2">
                <CheckCircle className="w-4 h-4 text-blue-400" />
                <span className="text-sm text-[#EAECEF]">平仓通知</span>
              </div>
              <button
                type="button"
                onClick={() => handleChange('notify_close', !settings.notify_close)}
                className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors ${
                  settings.notify_close ? 'bg-[#F0B90B]' : 'bg-[#2B3139]'
                }`}
              >
                <span
                  className={`inline-block h-3 w-3 transform rounded-full bg-white transition-transform ${
                    settings.notify_close ? 'translate-x-5' : 'translate-x-1'
                  }`}
                />
              </button>
            </div>

            <div className="flex items-center justify-between p-3 bg-[#1E2329] rounded">
              <div className="flex items-center gap-2">
                <AlertCircle className="w-4 h-4 text-red-400" />
                <span className="text-sm text-[#EAECEF]">错误通知</span>
              </div>
              <button
                type="button"
                onClick={() => handleChange('notify_errors', !settings.notify_errors)}
                className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors ${
                  settings.notify_errors ? 'bg-[#F0B90B]' : 'bg-[#2B3139]'
                }`}
              >
                <span
                  className={`inline-block h-3 w-3 transform rounded-full bg-white transition-transform ${
                    settings.notify_errors ? 'translate-x-5' : 'translate-x-1'
                  }`}
                />
              </button>
            </div>
          </div>

          {/* Actions */}
          <div className="flex gap-2 pt-2">
            <button
              type="button"
              onClick={handleSave}
              disabled={isSaving || !settings.chat_id}
              className="flex-1 px-4 py-2 bg-[#F0B90B] text-black rounded text-sm font-medium hover:bg-[#E1A706] transition-colors disabled:bg-[#848E9C] disabled:cursor-not-allowed flex items-center justify-center gap-2"
            >
              {isSaving ? (
                <Loader2 className="w-4 h-4 animate-spin" />
              ) : (
                'Save Settings'
              )}
            </button>
            <button
              type="button"
              onClick={handleTest}
              disabled={isTesting || !settings.chat_id}
              className="px-4 py-2 bg-[#2B3139] text-[#EAECEF] rounded text-sm hover:bg-[#404750] transition-colors disabled:bg-[#1E2329] disabled:text-[#848E9C] disabled:cursor-not-allowed flex items-center gap-2"
            >
              {isTesting ? (
                <Loader2 className="w-4 h-4 animate-spin" />
              ) : (
                <>
                  <Send className="w-4 h-4" />
                  Test
                </>
              )}
            </button>
          </div>
        </>
      )}
    </div>
  )
}
