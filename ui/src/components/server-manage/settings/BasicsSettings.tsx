'use client'

import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { useTranslations } from '@/contexts/LanguageContext'
import { SectionShell, PasswordInput, LaunchNumber, useServer } from '../shared'
import { useSettingsDraft } from '../SettingsDraftContext'
import { useParamText } from './paramText'

// Basics config page: server name / install path / port / passwords /
// description / REST API port. REST API is always enabled; only the port is
// configurable here. The game port (-port) is a launch argument set below.
export function BasicsSettings() {
  const t = useTranslations('serverConfig')
  const tm = useTranslations('serverManage')
  const { paramLabel } = useParamText()
  const { data: server } = useServer()
  const {
    name,
    installPath,
    launchArgs,
    installed,
    setName,
    setInstallPath,
    setSetting,
    setLaunch,
    iniValue,
  } = useSettingsDraft()

  const pathChanged = server ? installPath !== server.install_path : false

  return (
    <SectionShell title={tm('sections.basics')} desc={tm('basicsSection.desc')} comingSoon={false}>
      {!installed && <p className="text-sm text-warning">{t('notInstalledHint')}</p>}

      <div className="max-w-2xl space-y-5 rounded-2xl border-2 p-5 shadow-pal">
        <div className="space-y-2">
          <Label htmlFor="settings-name">{t('basics.name')}</Label>
          <Input id="settings-name" value={name} onChange={(e) => setName(e.target.value)} />
        </div>

        <div className="space-y-2">
          <Label htmlFor="settings-path">{t('basics.path')}</Label>
          <Input
            id="settings-path"
            value={installPath}
            onChange={(e) => setInstallPath(e.target.value)}
          />
          {pathChanged && <p className="text-sm text-warning">{t('basics.pathChangedHint')}</p>}
        </div>

        <LaunchNumber
          label={t('basics.port')}
          value={launchArgs.port}
          onChange={(v) => setLaunch({ port: v })}
          placeholder="8211"
        />

        <div className="space-y-2">
          <Label htmlFor="settings-serverpassword">{paramLabel('ServerPassword')}</Label>
          <PasswordInput
            id="settings-serverpassword"
            value={iniValue('ServerPassword')}
            onChange={(v) => setSetting('ServerPassword', v)}
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="settings-adminpassword">{paramLabel('AdminPassword')}</Label>
          <PasswordInput
            id="settings-adminpassword"
            value={iniValue('AdminPassword')}
            onChange={(v) => setSetting('AdminPassword', v)}
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="settings-serverdesc">{paramLabel('ServerDescription')}</Label>
          <Textarea
            id="settings-serverdesc"
            value={iniValue('ServerDescription')}
            onChange={(e) => setSetting('ServerDescription', e.target.value)}
            className="min-h-[72px]"
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="settings-restport">{paramLabel('RESTAPIPort')}</Label>
          <Input
            id="settings-restport"
            type="number"
            step="1"
            value={iniValue('RESTAPIPort')}
            onChange={(e) => setSetting('RESTAPIPort', e.target.value)}
            className="max-w-[220px]"
          />
        </div>
      </div>
    </SectionShell>
  )
}
