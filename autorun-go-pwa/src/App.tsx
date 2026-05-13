import { useEffect, useRef, useState } from 'react'
import { registerSW } from 'virtual:pwa-register'

type Status = 'loading' | 'empty' | 'ready'
type ActionStatus = 'idle' | 'loading' | 'success' | 'error'
type PageTab = 'run' | 'club' | 'mine'

interface BeforeInstallPromptEvent extends Event {
  prompt: () => Promise<void>
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed'; platform: string }>
}

interface ProgressCard {
  id: string
  title: string
  current: number
  target: number
  unit?: string
  subtitle: string
  accent: string
}

interface ActionItem {
  id: 'run' | 'club'
  title: string
  subtitle: string
}

interface Toast {
  id: string
  message: string
  tone: 'success' | 'error'
}

interface ClubActivityItem {
  id: string
  activityId: number
  title: string
  startTime: string
  endTime: string
  address: string
  joined: number
  capacity: number
  isJoined: boolean
  isFull: boolean
}

interface ClubSignTask {
  activityId: number
  activityName: string
  startTime: string
  endTime: string
  address: string
  latitude: string
  longitude: string
  signStatus: string
  signInStatus: string
  signBackStatus: string
  signInTime: string
  signBackLimitTime: number | null
}

interface ClubSignAction {
  signType: '1' | '2'
  label: string
  pendingLabel: string
  disabled: boolean
}

interface ClubScheduleState {
  enabled?: boolean
  lastMessage?: string
  lastProbeAt?: string
  lastActionAt?: string
}

const ACTIONS: ActionItem[] = [
  {
    id: 'run',
    title: '提交跑步记录',
    subtitle: '生成轨迹并提交记录'
  },
  {
    id: 'club',
    title: '俱乐部签到',
    subtitle: '自动签到或签退'
  }
]

const PAGE_META: Record<PageTab, { eyebrow: string; title: string; subtitle: string }> = {
  run: {
    eyebrow: '',
    title: '校园跑',
    subtitle: ''
  },
  club: {
    eyebrow: '',
    title: '俱乐部',
    subtitle: '查看签到状态并完成一键签到/签退'
  },
  mine: {
    eyebrow: '',
    title: '',
    subtitle: ''
  }
}

const API_BASE =
  import.meta.env.VITE_API_BASE ?? '/api'

const getApiEndpoint = (): string => {
  const base = String(API_BASE || '/api').trim()
  if (!base || base === '/') {
    return '/api'
  }
  return base.replace(/\/+$/, '')
}

const defaultActionState = ACTIONS.reduce<Record<string, { status: ActionStatus; message: string }>>(
  (acc, action) => {
    acc[action.id] = { status: 'idle', message: '' }
    return acc
  },
  {}
)

const asNumber = (value: unknown): number | null => {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value
  }
  if (typeof value === 'string') {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) {
      return parsed
    }
  }
  return null
}

const firstNumber = (obj: unknown, keys: string[]): number | null => {
  if (!obj || typeof obj !== 'object') {
    return null
  }
  const record = obj as Record<string, unknown>
  for (const key of keys) {
    const value = asNumber(record[key])
    if (value !== null) {
      return value
    }
  }
  return null
}

const maxNumber = (obj: unknown, keys: string[]): number | null => {
  if (!obj || typeof obj !== 'object') {
    return null
  }
  const record = obj as Record<string, unknown>
  const values = keys
    .map((key) => asNumber(record[key]))
    .filter((value): value is number => value !== null && value > 0)
  if (values.length === 0) {
    return null
  }
  return Math.max(...values)
}

const toKm = (distanceLike: number): number => {
  if (!Number.isFinite(distanceLike)) {
    return 0
  }
  if (Math.abs(distanceLike) >= 1000) {
    return distanceLike / 1000
  }
  return distanceLike
}

const formatDisplayNumber = (value: number): string => {
  if (Math.abs(value - Math.round(value)) < 1e-6) {
    return String(Math.round(value))
  }
  return value.toFixed(1)
}

const normalizeErrorMessage = (raw: unknown, fallback = '请求失败，请稍后重试'): string => {
  if (typeof raw !== 'string') {
    return fallback
  }
  const compact = raw
    .replace(/\\"/g, '"')
    .replace(/\\n/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
  if (!compact) {
    return fallback
  }

  const msgMatch = compact.match(/"msg"\s*:\s*"([^"]+)"/i)
  const messageFieldMatch = compact.match(/"Message"\s*:\s*"([^"]+)"/)
  const picked = (msgMatch?.[1] || messageFieldMatch?.[1] || compact).trim()
  if (picked.length > 80) {
    return `${picked.slice(0, 80)}...`
  }
  return picked
}

const isHTMLResponse = (raw: string): boolean => {
  const trimmed = raw.trim().toLowerCase()
  return trimmed.startsWith('<!doctype html') || trimmed.startsWith('<html')
}

const getTodayLocalDate = (): string => {
  const now = new Date()
  const year = now.getFullYear()
  const month = String(now.getMonth() + 1).padStart(2, '0')
  const day = String(now.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

const readStringField = (obj: unknown, key: string): string => {
  if (!obj || typeof obj !== 'object') {
    return ''
  }
  const value = (obj as Record<string, unknown>)[key]
  if (value === null || value === undefined) {
    return ''
  }
  return String(value).trim()
}

const isSignedStatus = (value: unknown): boolean => {
  return String(value ?? '').trim() === '1'
}

const normalizeClubSignTask = (raw: unknown): ClubSignTask | null => {
  if (!raw || typeof raw !== 'object') {
    return null
  }
  const activityId = firstNumber(raw, ['activityId', 'clubActivityId']) ?? 0
  if (activityId <= 0) {
    return null
  }
  return {
    activityId,
    activityName: readStringField(raw, 'activityName') || '未命名活动',
    startTime: readStringField(raw, 'startTime') || '--:--',
    endTime: readStringField(raw, 'endTime') || '--:--',
    address: readStringField(raw, 'address') || readStringField(raw, 'addressDetail') || '地点待公布',
    latitude: readStringField(raw, 'latitude'),
    longitude: readStringField(raw, 'longitude'),
    signStatus: readStringField(raw, 'signStatus'),
    signInStatus: readStringField(raw, 'signInStatus'),
    signBackStatus: readStringField(raw, 'signBackStatus'),
    signInTime: readStringField(raw, 'signInTime'),
    signBackLimitTime: firstNumber(raw, ['signBackLimitTime'])
  }
}

const resolveClubSignAction = (task: ClubSignTask | null): ClubSignAction | null => {
  if (!task) {
    return null
  }
  if (!isSignedStatus(task.signInStatus) && task.signStatus === '1') {
    return { signType: '1', label: '签到', pendingLabel: '签到中…', disabled: false }
  }
  if (isSignedStatus(task.signInStatus) && !isSignedStatus(task.signBackStatus)) {
    return {
      signType: '2',
      label: '签退',
      pendingLabel: '签退中…',
      disabled: task.signStatus !== '2'
    }
  }
  return null
}

const resolveClubSignTaskStatus = (task: ClubSignTask | null): string => {
  if (!task) {
    return '当前没有可执行签到/签退任务'
  }
  if (isSignedStatus(task.signInStatus) && isSignedStatus(task.signBackStatus)) {
    return '已完成签到/签退'
  }
  if (isSignedStatus(task.signInStatus)) {
    return task.signStatus === '2' ? '已签到，可签退' : '已签到，等待签退'
  }
  return task.signStatus === '1' ? '可签到' : '等待签到'
}

const parseLocalEventTime = (date: string, value: string): Date | null => {
  const raw = value.trim()
  if (!raw || raw === '--:--') {
    return null
  }
  const normalized = raw.replace(/\//g, '-').replace(' ', 'T')
  const fullDate = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}/.test(normalized)
    ? normalized
    : `${date}T${raw}`
  const parsed = new Date(fullDate)
  if (Number.isNaN(parsed.getTime())) {
    return null
  }
  return parsed
}

const formatCountdown = (ms: number): string => {
  const totalSeconds = Math.max(0, Math.floor(ms / 1000))
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const seconds = totalSeconds % 60
  if (hours > 0) {
    return `${String(hours).padStart(2, '0')}:${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`
  }
  return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`
}

const SESSION_KEY_STORAGE = 'autorun_session_key'
const APP_LIFECYCLE_HEARTBEAT_MS = 15_000

const createAppClientId = (): string => {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
    return crypto.randomUUID()
  }
  return `client-${Date.now()}-${Math.random().toString(16).slice(2)}`
}

const sendAppLifecycle = (clientId: string, event: 'open' | 'heartbeat' | 'close', keepalive = false) => {
  if (typeof window === 'undefined') {
    return
  }
  const payload = JSON.stringify({
    action: 'app_lifecycle',
    clientId,
    event
  })
  const endpoint = getApiEndpoint()
  if (keepalive && typeof navigator !== 'undefined' && typeof navigator.sendBeacon === 'function') {
    const sent = navigator.sendBeacon(endpoint, new Blob([payload], { type: 'application/json' }))
    if (sent) {
      return
    }
  }
  void fetch(endpoint, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: payload,
    keepalive
  }).catch(() => {})
}

export default function App() {
  const appClientIdRef = useRef(createAppClientId())
  const [status, setStatus] = useState<Status>('loading')
  const [cards, setCards] = useState<ProgressCard[]>([])
  const [runDataMessage, setRunDataMessage] = useState('请填写账号后刷新进度')
  const [needRefresh, setNeedRefresh] = useState(false)
  const [updateSW, setUpdateSW] = useState<(reload?: boolean) => void>(() => () => {})
  const [actionState, setActionState] = useState(defaultActionState)
  const [toasts, setToasts] = useState<Toast[]>([])
  const [phone, setPhone] = useState('')
  const [password, setPassword] = useState('')
  const [adminToken, setAdminToken] = useState('')
  const [sessionKey, setSessionKey] = useState(() => {
    if (typeof window === 'undefined') {
      return ''
    }
    return window.localStorage.getItem(SESSION_KEY_STORAGE) ?? ''
  })
  const [activeTab, setActiveTab] = useState<PageTab>('run')
  const [clubStatus, setClubStatus] = useState<Status>('loading')
  const [clubMessage, setClubMessage] = useState('请填写账号后刷新俱乐部数据')
  const [clubJoined, setClubJoined] = useState(0)
  const [clubTarget, setClubTarget] = useState(12)
  const [clubActivities, setClubActivities] = useState<ClubActivityItem[]>([])
  const [clubActionLoading, setClubActionLoading] = useState<Record<string, boolean>>({})
  const [clubQueryDate, setClubQueryDate] = useState(getTodayLocalDate())
  const [clubSignTask, setClubSignTask] = useState<ClubSignTask | null>(null)
  const [clubSignLoading, setClubSignLoading] = useState(false)
  const [clubScheduleEnabled, setClubScheduleEnabled] = useState(false)
  const [clubScheduleSaving, setClubScheduleSaving] = useState(false)
  const [clubScheduleMessage, setClubScheduleMessage] = useState('定时未开启')
  const [nowTick, setNowTick] = useState(() => Date.now())
  const [manualLoadingCount, setManualLoadingCount] = useState(0)
  const [authChecking, setAuthChecking] = useState(true)
  const [isAuthenticated, setIsAuthenticated] = useState(false)
  const [showLoginModal, setShowLoginModal] = useState(false)
  const [loginLoading, setLoginLoading] = useState(false)
  const [loginError, setLoginError] = useState('')
  const [installPromptEvent, setInstallPromptEvent] = useState<BeforeInstallPromptEvent | null>(null)
  const [canInstall, setCanInstall] = useState(false)

  useEffect(() => {
    const update = registerSW({
      onNeedRefresh() {
        setNeedRefresh(true)
        window.dispatchEvent(new CustomEvent('pwa:need-refresh'))
      },
      onOfflineReady() {
        window.dispatchEvent(new CustomEvent('pwa:offline-ready'))
      }
    })
    setUpdateSW(() => update)
  }, [])

  useEffect(() => {
    const clientId = appClientIdRef.current
    sendAppLifecycle(clientId, 'open')
    const heartbeatTimer = window.setInterval(() => {
      sendAppLifecycle(clientId, 'heartbeat')
    }, APP_LIFECYCLE_HEARTBEAT_MS)
    const notifyClose = () => {
      sendAppLifecycle(clientId, 'close', true)
    }

    window.addEventListener('pagehide', notifyClose)
    return () => {
      window.clearInterval(heartbeatTimer)
      window.removeEventListener('pagehide', notifyClose)
      notifyClose()
    }
  }, [])

  useEffect(() => {
    if (typeof window === 'undefined') {
      return
    }
    const onBeforeInstallPrompt = (event: Event) => {
      const installEvent = event as BeforeInstallPromptEvent
      installEvent.preventDefault()
      setInstallPromptEvent(installEvent)
      setCanInstall(true)
    }
    const onAppInstalled = () => {
      setInstallPromptEvent(null)
      setCanInstall(false)
      pushToast('安装成功，可在桌面打开应用', 'success')
    }
    window.addEventListener('beforeinstallprompt', onBeforeInstallPrompt)
    window.addEventListener('appinstalled', onAppInstalled)
    return () => {
      window.removeEventListener('beforeinstallprompt', onBeforeInstallPrompt)
      window.removeEventListener('appinstalled', onAppInstalled)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const requestJSON = async (
    payload: Record<string, unknown>,
    options?: { userInitiated?: boolean }
  ) => {
    const userInitiated = options?.userInitiated === true
    if (userInitiated) {
      setManualLoadingCount((prev) => prev + 1)
    }
    try {
    const endpoint = getApiEndpoint()
    const response = await fetch(endpoint, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    })
    const rawText = await response.text().catch(() => '')
    if (isHTMLResponse(rawText)) {
      throw new Error('请求没有进入后端 API，请确认从 AutoRun 启动入口打开页面')
    }
    let data: any = {}
    if (rawText) {
      try {
        data = JSON.parse(rawText)
        if (typeof data === 'string') {
          const trimmed = data.trim()
          if (trimmed.startsWith('{') || trimmed.startsWith('[')) {
            try {
              data = JSON.parse(trimmed)
            } catch {
              data = { msg: data }
            }
          } else {
            data = { msg: data }
          }
        }
      } catch {
        data = {}
      }
    }
    if (!response.ok) {
      if (response.status === 401) {
        setSessionKey('')
        setIsAuthenticated(false)
        setShowLoginModal(true)
        setAuthChecking(false)
        setPassword('')
        setLoginError('登录态已失效，请重新输入手机号和密码')
        if (typeof window !== 'undefined') {
          window.localStorage.removeItem(SESSION_KEY_STORAGE)
        }
      }
      const message =
        typeof data?.msg === 'string'
          ? data.msg
          : rawText
            ? rawText
            : `请求失败 (${response.status})`
      throw new Error(normalizeErrorMessage(message))
    }
    if (typeof data?.code === 'number' && data.code !== 10000) {
      throw new Error(normalizeErrorMessage(data?.msg ?? '操作失败'))
    }
    const nextSessionKey = data?.response?.sessionKey
    if (typeof nextSessionKey === 'string' && nextSessionKey.trim()) {
      const normalized = nextSessionKey.trim()
      setSessionKey(normalized)
      if (typeof window !== 'undefined') {
        window.localStorage.setItem(SESSION_KEY_STORAGE, normalized)
      }
    }
    return data
    } finally {
      if (userInitiated) {
        setManualLoadingCount((prev) => Math.max(0, prev - 1))
      }
    }
  }

  const applyAuthPayload = (payload: Record<string, unknown>) => {
    if (phone.trim()) {
      payload.phone = phone.trim()
    }
    if (password.trim()) {
      payload.password = password.trim()
    }
    if (adminToken.trim()) {
      payload.adminToken = adminToken.trim()
    }
    if (sessionKey.trim()) {
      payload.sessionKey = sessionKey.trim()
    }
  }

  const loadData = async (manual = false) => {
    setStatus('loading')
    try {
      const payload: Record<string, unknown> = { action: 'run_info' }
      applyAuthPayload(payload)

      const data = await requestJSON(payload, { userInitiated: manual })
      const runInfo = data?.response?.runInfo ?? {}
      const runStandard = data?.response?.runStandard ?? {}

      const currentCount = firstNumber(runInfo, ['runValidCount', 'runCount']) ?? 0
      const countTargetRaw =
        maxNumber(runStandard, [
          'boyAllRunTime',
          'girlAllRunTime',
          'allRunTime',
          'runTimes',
          'runCount',
          'targetRunCount',
          'minRunCount',
          'effectiveRunCount',
          'totalRunCount'
        ]) ?? 20
      const countTarget = Math.max(countTargetRaw, 1)

      const currentDistance = toKm(firstNumber(runInfo, ['runValidDistance', 'runDistance']) ?? 0)
      const distanceTargetRaw =
        maxNumber(runStandard, [
          'boyAllRunDistance',
          'girlAllRunDistance',
          'allRunDistance',
          'runDistance',
          'targetDistance',
          'minRunDistance',
          'effectiveDistance',
          'totalDistance'
        ]) ?? 60
      const distanceTarget = Math.max(toKm(distanceTargetRaw), 1)

      const nextCards: ProgressCard[] = [
        {
          id: 'count-progress',
          title: '校园跑次数进度',
          current: currentCount,
          target: countTarget,
          unit: '次',
          subtitle: '本学期有效打卡次数',
          accent: 'linear-gradient(90deg, #4f8cff, #55d6ff)'
        },
        {
          id: 'distance-progress',
          title: '校园跑距离进度',
          current: Number(currentDistance.toFixed(1)),
          target: Number(distanceTarget.toFixed(1)),
          unit: 'km',
          subtitle: '本学期累计有效距离',
          accent: 'linear-gradient(90deg, #ff9f43, #ffd166)'
        }
      ]

      setCards(nextCards)
      setRunDataMessage('已从后端同步校园跑进度')
      setStatus(nextCards.length === 0 ? 'empty' : 'ready')
    } catch (err) {
      setCards([])
      setStatus('empty')
      setRunDataMessage(normalizeErrorMessage(err instanceof Error ? err.message : '', '加载校园跑进度失败'))
    }
  }

  const loadClubData = async (manual = false) => {
    setClubStatus('loading')
    try {
      const payload: Record<string, unknown> = { action: 'club_data' }
      if (clubQueryDate) {
        payload.queryDate = clubQueryDate
      }
      applyAuthPayload(payload)

	      const data = await requestJSON(payload, { userInitiated: manual })
	      const joinProgress = data?.response?.joinProgress ?? {}
	      const joined = firstNumber(joinProgress, ['joinNum']) ?? 0
	      const targetRaw = firstNumber(joinProgress, ['totalNum']) ?? 12
	      const target = Math.max(targetRaw, joined, 1)
	      const signTask = normalizeClubSignTask(data?.response?.signTask)
	      const schedule = (data?.response?.schedule ?? {}) as ClubScheduleState

	      const rawActivities = Array.isArray(data?.response?.activities) ? data.response.activities : []
      const mappedActivities: ClubActivityItem[] = rawActivities.map((item: any, index: number) => {
        const startTime = typeof item?.startTime === 'string' ? item.startTime : '--:--'
        const endTime = typeof item?.endTime === 'string' ? item.endTime : '--:--'
        const joinedCount = firstNumber(item, ['signInStudent', 'applyStudentCount']) ?? 0
        const capacity = firstNumber(item, ['maxStudent']) ?? 0
        const optionStatus = String(item?.optionStatus ?? '').trim()
        const fullFlag = String(item?.fullActivity ?? '')
        // Club optionStatus codes: 6=可报名, 1=已报名, 3=已报满.
        const isJoined = optionStatus === '1'
        const isFull = optionStatus === '3' || fullFlag === '1' || (capacity > 0 && joinedCount >= capacity)

        return {
          id: String(item?.clubActivityId ?? item?.activityId ?? index),
          activityId: Number(item?.clubActivityId ?? item?.activityId ?? 0),
          title: typeof item?.activityName === 'string' && item.activityName ? item.activityName : '未命名活动',
          startTime,
          endTime,
          address:
            typeof item?.addressDetail === 'string' && item.addressDetail
              ? item.addressDetail
              : typeof item?.teacherName === 'string' && item.teacherName
                ? item.teacherName
                : '地点待公布',
          joined: joinedCount,
          capacity,
          isJoined,
          isFull
        }
      })

	      setClubJoined(joined)
	      setClubTarget(target)
	      setClubSignTask(signTask)
	      setClubScheduleEnabled(schedule.enabled === true)
	      setClubScheduleMessage(
	        typeof schedule.lastMessage === 'string' && schedule.lastMessage
	          ? schedule.lastMessage
	          : schedule.enabled
	            ? '定时已开启'
	            : '定时未开启'
	      )
	      setClubActivities(mappedActivities)
	      setClubStatus(mappedActivities.length === 0 ? 'empty' : 'ready')
	      setClubMessage(`已同步 ${clubQueryDate} 的俱乐部活动`)
	    } catch (err) {
	      setClubActivities([])
	      setClubSignTask(null)
	      setClubStatus('empty')
	      setClubMessage(normalizeErrorMessage(err instanceof Error ? err.message : '', '加载俱乐部数据失败'))
	    }
  }

  const verifyAuthOnEnter = async () => {
    setAuthChecking(true)
    setLoginError('')
    const savedSession = sessionKey.trim()
    if (!savedSession) {
      setIsAuthenticated(false)
      setShowLoginModal(true)
      setAuthChecking(false)
      return
    }

    try {
      await requestJSON({ action: 'session_bootstrap', sessionKey: savedSession })
      setIsAuthenticated(true)
      setShowLoginModal(false)
      await loadData()
    } catch {
      setIsAuthenticated(false)
      setShowLoginModal(true)
      setLoginError('登录态已过期，请重新登录')
      setPassword('')
    } finally {
      setAuthChecking(false)
    }
  }

  useEffect(() => {
    void verifyAuthOnEnter()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    const timer = window.setInterval(() => {
      setNowTick(Date.now())
    }, 1000)
    return () => window.clearInterval(timer)
  }, [])

  useEffect(() => {
    if (isAuthenticated && !authChecking && activeTab === 'club') {
      void loadClubData()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeTab, clubQueryDate, isAuthenticated, authChecking])

  const ensureAuthenticated = () => {
    if (isAuthenticated) {
      return true
    }
    setShowLoginModal(true)
    if (!loginError) {
      setLoginError('请先登录后再进行操作')
    }
    return false
  }

  const handleRefresh = () => {
    updateSW(true)
    setNeedRefresh(false)
  }

  const pushToast = (message: string, tone: Toast['tone']) => {
    const id =
      typeof crypto !== 'undefined' && 'randomUUID' in crypto
        ? crypto.randomUUID()
        : `${Date.now()}-${Math.random()}`
    setToasts((prev) => [...prev, { id, message, tone }])
    window.setTimeout(() => {
      setToasts((prev) => prev.filter((toast) => toast.id !== id))
    }, 3200)
  }

  const handleInstallApp = async () => {
    const ua = typeof navigator !== 'undefined' ? navigator.userAgent.toLowerCase() : ''
    const isIOS = /iphone|ipad|ipod/.test(ua)
    const isStandalone =
      typeof window !== 'undefined' &&
      (window.matchMedia('(display-mode: standalone)').matches ||
        ((window.navigator as Navigator & { standalone?: boolean }).standalone === true))

    if (isIOS && !isStandalone) {
      pushToast('iPhone请点“分享”后选择“添加到主屏幕”', 'success')
      return
    }

    if (!installPromptEvent) {
      pushToast('当前环境暂不支持安装，请稍后再试', 'error')
      return
    }

    await installPromptEvent.prompt()
    const choice = await installPromptEvent.userChoice
    if (choice.outcome === 'accepted') {
      setCanInstall(false)
      setInstallPromptEvent(null)
    }
  }

  const runAction = async (action: ActionItem) => {
    if (!ensureAuthenticated()) {
      return
    }
    setActionState((prev) => ({
      ...prev,
      [action.id]: { status: 'loading', message: '处理中…' }
    }))
    try {
      const payload: Record<string, unknown> = { action: action.id }
      applyAuthPayload(payload)

      const data = await requestJSON(payload, { userInitiated: true })
      const activityName = data?.response?.activityName as string | undefined
      const backendMsg = typeof data?.msg === 'string' && data.msg.trim() ? data.msg.trim() : '操作成功'
      const successMessage =
        action.id === 'club' && activityName ? `${backendMsg}：${activityName}` : backendMsg
      setActionState((prev) => ({
        ...prev,
        [action.id]: { status: 'success', message: successMessage }
      }))
      pushToast(successMessage, 'success')
      if (action.id === 'run') {
        void loadData()
      }
      if (action.id === 'club') {
        void loadClubData()
      }
    } catch (err) {
      const errorMessage = normalizeErrorMessage(err instanceof Error ? err.message : '', '请求失败')
      setActionState((prev) => ({
        ...prev,
        [action.id]: { status: 'error', message: errorMessage }
      }))
      pushToast(errorMessage, 'error')
    }
  }

  const toggleClubJoin = async (activity: ClubActivityItem) => {
    if (!ensureAuthenticated()) {
      return
    }
    if (!activity.activityId) {
      pushToast('活动ID无效', 'error')
      return
    }
    const action = activity.isJoined ? 'club_cancel' : 'club_join'
    setClubActionLoading((prev) => ({ ...prev, [activity.id]: true }))
    try {
      const payload: Record<string, unknown> = {
        action,
        activityId: activity.activityId
      }
      applyAuthPayload(payload)
      await requestJSON(payload, { userInitiated: true })
      pushToast(activity.isJoined ? '已取消报名' : '报名成功', 'success')
      await loadClubData(true)
    } catch (err) {
      pushToast(normalizeErrorMessage(err instanceof Error ? err.message : '', '操作失败'), 'error')
    } finally {
      setClubActionLoading((prev) => ({ ...prev, [activity.id]: false }))
    }
  }

  const handleClubSign = async () => {
    if (!ensureAuthenticated()) {
      return
    }
    const action = resolveClubSignAction(clubSignTask)
    if (!clubSignTask || !action) {
      pushToast('当前没有可操作的签到/签退任务', 'error')
      return
    }
    if (action.disabled) {
      pushToast('还没到可签退时间，请等待倒计时结束后刷新', 'error')
      return
    }

    setClubSignLoading(true)
    try {
      const payload: Record<string, unknown> = {
        action: 'club_sign',
        signType: action.signType
      }
      applyAuthPayload(payload)
      const data = await requestJSON(payload, { userInitiated: true })
      if (data?.response?.success !== true) {
        throw new Error(typeof data?.msg === 'string' ? data.msg : '当前暂不可操作')
      }

      const successText = action.signType === '1' ? '签到成功' : '签退成功'
      setClubSignTask((prev) => {
        if (!prev) {
          return prev
        }
        if (action.signType === '1') {
          return { ...prev, signInStatus: '1', signStatus: '2' }
        }
        return { ...prev, signBackStatus: '1' }
      })
      pushToast(successText, 'success')
      await loadClubData(true)
    } catch (err) {
      pushToast(normalizeErrorMessage(err instanceof Error ? err.message : '', '操作失败'), 'error')
    } finally {
      setClubSignLoading(false)
    }
  }

  const saveClubScheduleEnabled = async (enabled: boolean) => {
    if (!ensureAuthenticated()) {
      return
    }

    setClubScheduleSaving(true)
    try {
      const payload: Record<string, unknown> = {
        action: 'club_schedule_set',
        enabled
      }
      applyAuthPayload(payload)
      const data = await requestJSON(payload, { userInitiated: true })
      const schedule = (data?.response?.schedule ?? {}) as ClubScheduleState
      setClubScheduleEnabled(schedule.enabled === true)
      setClubScheduleMessage(
        typeof schedule.lastMessage === 'string' && schedule.lastMessage
          ? schedule.lastMessage
          : enabled
            ? '定时已开启'
            : '定时未开启'
      )
      pushToast(typeof data?.msg === 'string' ? data.msg : enabled ? '已开启俱乐部定时' : '已关闭俱乐部定时', 'success')
    } catch (err) {
      pushToast(normalizeErrorMessage(err instanceof Error ? err.message : '', '保存定时配置失败'), 'error')
    } finally {
      setClubScheduleSaving(false)
    }
  }

  const renderActionCard = (actionId: ActionItem['id']) => {
    const action = ACTIONS.find((item) => item.id === actionId)
    if (!action) {
      return null
    }
    const state = actionState[action.id]

    return (
      <button
        key={action.id}
        className={`action-card ${state.status}`}
        onClick={() => runAction(action)}
        disabled={state.status === 'loading'}
      >
        <div className="action-info">
          <p className="action-title">{action.title}</p>
          <p className="action-sub">{action.subtitle}</p>
          {state.message && <p className={`action-status ${state.status}`}>{state.message}</p>}
        </div>
        <div className="action-cta">{state.status === 'loading' ? '处理中…' : '开始'}</div>
      </button>
    )
  }

  const renderCredentials = () => (
    <div className="credentials glass">
      <div className="credentials-header">
        <h3>unirun账号</h3>
        <span>必填</span>
      </div>
      <div className="credentials-grid">
        <label>
          <span>手机号</span>
          <input
            placeholder="普通用户必填"
            value={phone}
            onChange={(event) => setPhone(event.target.value)}
          />
        </label>
        <label>
          <span>密码</span>
          <input
            type="password"
            placeholder="普通用户必填"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />
        </label>
        <label>
          <span>管理员口令（仅你本人）</span>
          <input
            type="password"
            placeholder="设置 ADMIN_TOKEN 后可免填账号"
            value={adminToken}
            onChange={(event) => setAdminToken(event.target.value)}
          />
        </label>
      </div>
      <p className="credentials-tip">普通用户必须输入账号密码；管理员口令通过后端 ADMIN_TOKEN 验证。</p>
    </div>
  )

  const handleModalLogin = async () => {
    const nextPhone = phone.trim()
    const nextPassword = password.trim()
    if (!nextPhone || !nextPassword) {
      setLoginError('手机号和密码不能为空')
      return
    }

    setLoginLoading(true)
    setLoginError('')
    try {
      await requestJSON({
        action: 'login',
        phone: nextPhone,
        password: nextPassword
      })
      setIsAuthenticated(true)
      setShowLoginModal(false)
      setPassword(nextPassword)
      pushToast('登录成功', 'success')
      await loadData()
      if (activeTab === 'club') {
        await loadClubData()
      }
    } catch (err) {
      setIsAuthenticated(false)
      setShowLoginModal(true)
      setLoginError(normalizeErrorMessage(err instanceof Error ? err.message : '', '登录失败'))
    } finally {
      setLoginLoading(false)
      setAuthChecking(false)
    }
  }

  const pageMeta = PAGE_META[activeTab]
  const clubRate = Math.max(0, Math.min(100, (clubJoined / Math.max(clubTarget, 1)) * 100))
  const weekLabels = ['一', '二', '三', '四', '五', '六', '日']
  const weekDates = Array.from({ length: 7 }).map((_, index) => {
    const date = new Date()
    date.setDate(date.getDate() + index)
    const full = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(
      date.getDate()
    ).padStart(2, '0')}`
    return {
      day: weekLabels[(date.getDay() + 6) % 7],
      date: String(date.getDate()).padStart(2, '0'),
      full,
      active: full === clubQueryDate
    }
  })
  const clubSignAction = resolveClubSignAction(clubSignTask)
  const clubSignStatusText = resolveClubSignTaskStatus(clubSignTask)
  const signBackProbeAt = clubSignTask ? parseLocalEventTime(getTodayLocalDate(), clubSignTask.endTime) : null
  const signBackCountdownMs =
    signBackProbeAt && isSignedStatus(clubSignTask?.signInStatus) && !isSignedStatus(clubSignTask?.signBackStatus)
      ? signBackProbeAt.getTime() - 10 * 60 * 1000 - nowTick
      : null
  const signBackCountdownText =
    signBackCountdownMs === null
      ? ''
      : signBackCountdownMs > 0
        ? `距签退窗口 ${formatCountdown(signBackCountdownMs)}`
        : '已进入签退试探窗口'

  return (
    <div className="app">
      {toasts.length > 0 && (
        <div className="toast-stack" role="status" aria-live="polite">
          {toasts.map((toast) => (
            <div key={toast.id} className={`toast glass ${toast.tone}`}>
              {toast.message}
            </div>
          ))}
        </div>
      )}

      <header className={`header ${activeTab === 'club' ? 'club-header' : ''}`}>
        <div className="title-block">
          <p className="eyebrow">{pageMeta.eyebrow}</p>
          <h1>{pageMeta.title}</h1>
          <p className="subtitle">{pageMeta.subtitle}</p>
        </div>
      </header>

      <main className="body">
        {activeTab === 'run' && (
          <>
            <section className="cards">
              {status === 'loading' && (
                <>
                  {[0, 1, 2].map((key) => (
                    <div className="card skeleton" key={key}>
                      <div className="skeleton-icon" />
                      <div className="skeleton-lines">
                        <span />
                        <span />
                        <span />
                      </div>
                    </div>
                  ))}
                </>
              )}

              {status === 'empty' && (
                <div className="empty glass">
                  <h2>暂无数据</h2>
                  <p>网络连接正常后会自动更新</p>
                  <button className="secondary" onClick={() => void loadData(true)}>
                    重新加载
                  </button>
                </div>
              )}

              {status === 'ready' &&
                cards.map((card) => (
                  <div className="card" key={card.id}>
                    <div className="card-icon" style={{ background: card.accent }}>
                      <span>{card.title.slice(0, 1)}</span>
                    </div>
                    <div className="card-body">
                      <p className="card-title">{card.title}</p>
                      <p className="card-value">
                        {formatDisplayNumber(card.current)}/{formatDisplayNumber(card.target)}
                        {card.unit && <span>{card.unit}</span>}
                      </p>
                      <div className="skill-bar" aria-label={`${card.title}进度`}>
                        <span
                          className="skill-per"
                          style={{
                            width: `${Math.max(0, Math.min(100, (card.current / card.target) * 100))}%`,
                            background: card.accent
                          }}
                        >
                          <span className="tooltip">
                            {Math.round(Math.max(0, Math.min(100, (card.current / card.target) * 100)))}%
                          </span>
                        </span>
                      </div>
                      <p className="card-subtitle">{card.subtitle}</p>
                    </div>
                  </div>
                ))}
            </section>

            <section className="actions">
              <div className="actions-header">
                <h2>校园跑</h2>
                <p>{runDataMessage}</p>
              </div>
              <button className="secondary refresh-btn" onClick={() => void loadData(true)}>
                刷新进度
              </button>
              <div className="actions-grid">{renderActionCard('run')}</div>
            </section>
          </>
        )}

        {activeTab === 'club' && (
          <section className="club-page">
            <div className="club-hero">
              <p>运动让生活更美好</p>
              <div className="club-loader">
                <div className="club-truck-wrapper">
                  <div className="club-truck-body">
                    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 198 93" className="club-truck-svg">
                      <path strokeWidth={3} stroke="#282828" fill="#F83D3D" d="M135 22.5H177.264C178.295 22.5 179.22 23.133 179.594 24.0939L192.33 56.8443C192.442 57.1332 192.5 57.4404 192.5 57.7504V89C192.5 90.3807 191.381 91.5 190 91.5H135C133.619 91.5 132.5 90.3807 132.5 89V25C132.5 23.6193 133.619 22.5 135 22.5Z" />
                      <path strokeWidth={3} stroke="#282828" fill="#7D7C7C" d="M146 33.5H181.741C182.779 33.5 183.709 34.1415 184.078 35.112L190.538 52.112C191.16 53.748 189.951 55.5 188.201 55.5H146C144.619 55.5 143.5 54.3807 143.5 53V36C143.5 34.6193 144.619 33.5 146 33.5Z" />
                      <path strokeWidth={2} stroke="#282828" fill="#282828" d="M150 65C150 65.39 149.763 65.8656 149.127 66.2893C148.499 66.7083 147.573 67 146.5 67C145.427 67 144.501 66.7083 143.873 66.2893C143.237 65.8656 143 65.39 143 65C143 64.61 143.237 64.1344 143.873 63.7107C144.501 63.2917 145.427 63 146.5 63C147.573 63 148.499 63.2917 149.127 63.7107C149.763 64.1344 150 64.61 150 65Z" />
                      <rect strokeWidth={2} stroke="#282828" fill="#FFFCAB" rx={1} height={7} width={5} y={63} x={187} />
                      <rect strokeWidth={2} stroke="#282828" fill="#282828" rx={1} height={11} width={4} y={81} x={193} />
                      <rect strokeWidth={3} stroke="#282828" fill="#DFDFDF" rx="2.5" height={90} width={121} y="1.5" x="6.5" />
                      <rect strokeWidth={2} stroke="#282828" fill="#DFDFDF" rx={2} height={4} width={6} y={84} x={1} />
                    </svg>
                  </div>
                  <div className="club-truck-tires">
                    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 30 30">
                      <circle strokeWidth={3} stroke="#282828" fill="#282828" r="13.5" cy={15} cx={15} />
                      <circle fill="#DFDFDF" r={7} cy={15} cx={15} />
                    </svg>
                    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 30 30">
                      <circle strokeWidth={3} stroke="#282828" fill="#282828" r="13.5" cy={15} cx={15} />
                      <circle fill="#DFDFDF" r={7} cy={15} cx={15} />
                    </svg>
                  </div>
                  <div className="club-road" />
                  <svg xmlSpace="preserve" viewBox="0 0 453.459 453.459" xmlnsXlink="http://www.w3.org/1999/xlink" xmlns="http://www.w3.org/2000/svg" id="Capa_1" version="1.1" fill="#000000" className="club-lamp-post">
                    <path d="M252.882,0c-37.781,0-68.686,29.953-70.245,67.358h-6.917v8.954c-26.109,2.163-45.463,10.011-45.463,19.366h9.993
      c-1.65,5.146-2.507,10.54-2.507,16.017c0,28.956,23.558,52.514,52.514,52.514c28.956,0,52.514-23.558,52.514-52.514
      c0-5.478-0.856-10.872-2.506-16.017h9.992c0-9.354-19.352-17.204-45.463-19.366v-8.954h-6.149C200.189,38.779,223.924,16,252.882,16
      c29.952,0,54.32,24.368,54.32,54.32c0,28.774-11.078,37.009-25.105,47.437c-17.444,12.968-37.216,27.667-37.216,78.884v113.914
      h-0.797c-5.068,0-9.174,4.108-9.174,9.177c0,2.844,1.293,5.383,3.321,7.066c-3.432,27.933-26.851,95.744-8.226,115.459v11.202h45.75
      v-11.202c18.625-19.715-4.794-87.527-8.227-115.459c2.029-1.683,3.322-4.223,3.322-7.066c0-5.068-4.107-9.177-9.176-9.177h-0.795
      V196.641c0-43.174,14.942-54.283,30.762-66.043c14.793-10.997,31.559-23.461,31.559-60.277C323.202,31.545,291.656,0,252.882,0z
      M232.77,111.694c0,23.442-19.071,42.514-42.514,42.514c-23.442,0-42.514-19.072-42.514-42.514c0-5.531,1.078-10.957,3.141-16.017
      h78.747C231.693,100.736,232.77,106.162,232.77,111.694z" />
                  </svg>
                </div>
              </div>
            </div>

            <div className="club-progress">
              <div className="club-progress-track">
                <span className="club-progress-fill" style={{ width: `${clubRate}%` }} />
              </div>
              <div className="club-progress-meta">
                <span>已参加：{clubJoined}次</span>
                <span>目标：{clubTarget}次</span>
              </div>
            </div>

            <div className="club-sign-panel">
              <div className="club-sign-main">
                <div>
                  <span className={`club-sign-badge ${clubSignAction?.signType === '2' ? 'signback' : 'signin'}`}>
                    {clubSignStatusText}
                  </span>
                  <h3>{clubSignTask?.activityName ?? '签到任务'}</h3>
                  {clubSignTask ? (
                    <>
                      <p>活动时间：{clubSignTask.startTime}-{clubSignTask.endTime}</p>
                      <p>活动地点：{clubSignTask.address}</p>
                    </>
                  ) : (
                    <p>{clubMessage}</p>
                  )}
                </div>
                <button
                  className={`club-sign-btn ${clubSignAction?.signType === '2' ? 'signback' : 'signin'}`}
                  type="button"
                  disabled={!clubSignAction || clubSignAction.disabled || clubSignLoading}
                  onClick={() => void handleClubSign()}
                >
                  {clubSignLoading ? clubSignAction?.pendingLabel ?? '处理中…' : clubSignAction?.label ?? '暂无任务'}
                </button>
              </div>

              {clubSignTask && (
                <div className="club-sign-detail">
                  <span>签到：{clubSignTask.signInTime || (isSignedStatus(clubSignTask.signInStatus) ? '已签到' : '--')}</span>
                  <span>{signBackCountdownText || `签退：${isSignedStatus(clubSignTask.signBackStatus) ? '已签退' : '--'}`}</span>
                </div>
              )}
            </div>

            <div className="club-schedule-panel">
              <div>
                <h3>定时签到/签退</h3>
                <p>{clubScheduleMessage}</p>
              </div>
              <label className={`club-switch ${clubScheduleSaving ? 'saving' : ''}`}>
                <input
                  type="checkbox"
                  checked={clubScheduleEnabled}
                  disabled={clubScheduleSaving}
                  onChange={(event) => void saveClubScheduleEnabled(event.target.checked)}
                />
                <span />
              </label>
            </div>

            <div className="club-board">
              <div className="club-board-head">
                <h3>俱乐部活动</h3>
                <button className="club-link" type="button">
                  点日期查询
                </button>
              </div>

              <div className="club-calendar">
                {weekDates.map((item) => (
                  <button
                    key={`${item.day}-${item.date}`}
                    type="button"
                    className={`club-day ${item.active ? 'active' : ''}`}
                    onClick={() => setClubQueryDate(item.full)}
                  >
                    <span>{item.day}</span>
                    <strong>{item.date}</strong>
                  </button>
                ))}
              </div>

              {clubStatus === 'loading' && (
                <div className="club-empty">
                  <p>正在同步俱乐部活动…</p>
                </div>
              )}

              {clubStatus === 'empty' && (
                <div className="club-empty">
                  <p>{clubMessage}</p>
                </div>
              )}

              {clubStatus === 'ready' && (
                <div className="club-list">
                  {clubActivities.map((activity) => (
                    <article className="club-item" key={activity.id}>
                      <h4>{activity.title}</h4>
                      <p>活动时间：{activity.startTime}-{activity.endTime}</p>
                      <p>活动地点：{activity.address}</p>
                      <div className="club-item-meta">
                        <div className="club-item-meta-left">
                          <span>体能教研室</span>
                          <span>
                            {activity.joined} / {activity.capacity || '--'} 人
                          </span>
                        </div>
                        <button
                          className={`club-join-btn ${activity.isJoined ? 'cancel' : 'join'}`}
                          disabled={clubActionLoading[activity.id] || (!activity.isJoined && activity.isFull)}
                          onClick={() => void toggleClubJoin(activity)}
                        >
                          {clubActionLoading[activity.id]
                            ? '处理中'
                            : activity.isJoined
                              ? '取消报名'
                              : activity.isFull
                                ? '已满员'
                                : '报名'}
                        </button>
                      </div>
                    </article>
                  ))}
                </div>
              )}
            </div>

            <button className="secondary refresh-btn" onClick={() => void loadClubData(true)}>
              刷新活动
            </button>
          </section>
        )}

        {activeTab === 'mine' && (
          <section className="actions">
            <div className="actions-header">
              <h2>我的</h2>
              <p>账号信息仅用于当前会话请求，不会在前端持久化。</p>
            </div>
            <button className="primary install-btn" onClick={() => void handleInstallApp()}>
              安装应用
            </button>
            {renderCredentials()}
            
          </section>
        )}
      </main>

      <footer className="footer-nav-wrap">
        <div className="glass-radio-group">
          <input
            type="radio"
            name="page"
            id="tab-run"
            checked={activeTab === 'run'}
            onChange={() => setActiveTab('run')}
          />
          <label htmlFor="tab-run">校园跑</label>

          <input
            type="radio"
            name="page"
            id="tab-club"
            checked={activeTab === 'club'}
            onChange={() => setActiveTab('club')}
          />
          <label htmlFor="tab-club">俱乐部</label>

          <input
            type="radio"
            name="page"
            id="tab-mine"
            checked={activeTab === 'mine'}
            onChange={() => setActiveTab('mine')}
          />
          <label htmlFor="tab-mine">我的</label>

          <div className="glass-glider" />
        </div>
      </footer>

      {needRefresh && (
        <div className="update-banner glass" role="status" aria-live="polite">
          <div>
            <p className="update-title">发现新版本</p>
            <p className="update-sub">请点击刷新以获取最新内容</p>
          </div>
          <button className="primary" onClick={handleRefresh}>
            刷新
          </button>
        </div>
      )}

      {canInstall && activeTab !== 'mine' && (
        <button className="install-fab" onClick={() => void handleInstallApp()}>
          安装应用
        </button>
      )}

      {manualLoadingCount > 0 && (
        <div className="request-loader-overlay" role="status" aria-live="polite" aria-label="请求处理中">
          <div className="request-loader-card">
            <div
              aria-label="Orange and tan hamster running in a metal wheel"
              role="img"
              className="wheel-and-hamster"
            >
              <div className="wheel" />
              <div className="hamster">
                <div className="hamster__body">
                  <div className="hamster__head">
                    <div className="hamster__ear" />
                    <div className="hamster__eye" />
                    <div className="hamster__nose" />
                  </div>
                  <div className="hamster__limb hamster__limb--fr" />
                  <div className="hamster__limb hamster__limb--fl" />
                  <div className="hamster__limb hamster__limb--br" />
                  <div className="hamster__limb hamster__limb--bl" />
                  <div className="hamster__tail" />
                </div>
              </div>
              <div className="spoke" />
            </div>
            <p className="request-loader-text">请求处理中...</p>
          </div>
        </div>
      )}

      {showLoginModal && (
        <div className="auth-modal-backdrop" role="dialog" aria-modal="true" aria-label="登录验证">
          <div className="auth-modal glass">
            <h3>登录已失效</h3>
            <p>请输入手机号和密码，完成后继续使用。</p>
            <label>
              <span>手机号</span>
              <input
                value={phone}
                onChange={(event) => setPhone(event.target.value)}
                placeholder="请输入手机号"
                autoComplete="username"
              />
            </label>
            <label>
              <span>密码</span>
              <input
                type="password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                placeholder="请输入密码"
                autoComplete="current-password"
                onKeyDown={(event) => {
                  if (event.key === 'Enter') {
                    void handleModalLogin()
                  }
                }}
              />
            </label>
            {loginError && <p className="auth-modal-error">{loginError}</p>}
            <button className="primary auth-modal-submit" onClick={() => void handleModalLogin()} disabled={loginLoading}>
              {loginLoading || authChecking ? '验证中...' : '登录'}
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
