import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setLocale } from '../../i18n'
import WebhookEnvVarsCard from './WebhookEnvVarsCard.vue'

describe('WebhookEnvVarsCard', () => {
  beforeEach(() => {
    setLocale('fr')
  })

  it('renders the translated label plus each env var name/description', () => {
    const wrapper = mount(WebhookEnvVarsCard, {
      props: {
        envVars: [
          { name: 'SS_REPO_NAME', desc: 'owner/repo (ex: home-assistant/core)' },
          { name: 'SS_TAG_NAME', desc: 'Tag de la nouvelle release (ex: v1.2.3)' },
        ],
      },
    })
    expect(wrapper.text()).toContain('Variables injectées dans votre script :')
    expect(wrapper.text()).toContain('SS_REPO_NAME')
    expect(wrapper.text()).toContain('owner/repo (ex: home-assistant/core)')
  })

  it('translates the label to English when the locale is switched', () => {
    setLocale('en')
    const wrapper = mount(WebhookEnvVarsCard, { props: { envVars: [] } })
    expect(wrapper.text()).toContain('Variables injected into your script:')
  })
})
