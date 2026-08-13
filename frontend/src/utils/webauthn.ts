// Minimal WebAuthn browser-API glue. The server (go-webauthn) speaks the
// spec's JSON dialect directly — challenge/id fields as base64url strings —
// but navigator.credentials.create()/get() require real ArrayBuffers for
// those same fields, and the PublicKeyCredential they return has to be
// converted back to base64url before it's sent back to the server. Hand-rolled
// rather than pulling in a browser helper library, since this conversion is
// the entire integration surface and keeping it in one place avoids a second
// library's JSON dialect potentially drifting from go-webauthn's.

// Exported (not just used internally) so their round-trip can be unit tested
// without mocking navigator.credentials — a padding/charset bug here would
// silently break every WebAuthn ceremony's challenge/id fields.
export function base64urlToBuffer(value: string): ArrayBuffer {
  const padded = value.replace(/-/g, '+').replace(/_/g, '/').padEnd(Math.ceil(value.length / 4) * 4, '=')
  const raw = atob(padded)
  const buffer = new Uint8Array(raw.length)
  for (let i = 0; i < raw.length; i++) buffer[i] = raw.charCodeAt(i)
  return buffer.buffer
}

export function bufferToBase64url(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer)
  let str = ''
  for (let i = 0; i < bytes.length; i++) str += String.fromCharCode(bytes[i])
  return btoa(str).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

interface ServerDescriptor {
  id: string
  type: string
  transports?: string[]
}

// Shapes of the go-webauthn protocol.CredentialCreation/CredentialAssertion
// JSON this app's /webauthn/*/begin endpoints return — only the base64url
// fields that need decoding are named; everything else (rp, pubKeyCredParams,
// timeout, authenticatorSelection, attestation...) passes through untouched.
interface ServerCreationOptions {
  publicKey: {
    challenge: string
    user: { id: string; [key: string]: unknown }
    excludeCredentials?: ServerDescriptor[]
    [key: string]: unknown
  }
}

interface ServerRequestOptions {
  publicKey: {
    challenge: string
    allowCredentials?: ServerDescriptor[]
    [key: string]: unknown
  }
}

// Walks the server's CredentialCreation/CredentialAssertion JSON and decodes
// every base64url-encoded field the browser API expects as an ArrayBuffer.
function decodeCreationOptions(options: ServerCreationOptions): CredentialCreationOptions {
  const publicKey: Record<string, unknown> = { ...options.publicKey }
  publicKey.challenge = base64urlToBuffer(options.publicKey.challenge)
  publicKey.user = { ...options.publicKey.user, id: base64urlToBuffer(options.publicKey.user.id) }
  if (Array.isArray(options.publicKey.excludeCredentials)) {
    publicKey.excludeCredentials = options.publicKey.excludeCredentials.map((c) => ({
      ...c,
      id: base64urlToBuffer(c.id),
    }))
  }
  return { publicKey } as unknown as CredentialCreationOptions
}

function decodeRequestOptions(options: ServerRequestOptions): CredentialRequestOptions {
  const publicKey: Record<string, unknown> = { ...options.publicKey }
  publicKey.challenge = base64urlToBuffer(options.publicKey.challenge)
  if (Array.isArray(options.publicKey.allowCredentials)) {
    publicKey.allowCredentials = options.publicKey.allowCredentials.map((c) => ({
      ...c,
      id: base64urlToBuffer(c.id),
    }))
  }
  return { publicKey } as unknown as CredentialRequestOptions
}

// Encodes a freshly created/asserted PublicKeyCredential back into the plain
// JSON shape protocol.ParseCredentialCreationResponseBody/
// ParseCredentialRequestResponseBody expect server-side.
function encodeCredential(credential: PublicKeyCredential): Record<string, unknown> {
  const response = credential.response
  const base = {
    id: credential.id,
    rawId: bufferToBase64url(credential.rawId),
    type: credential.type,
    clientExtensionResults: credential.getClientExtensionResults?.() ?? {},
  }
  if (response instanceof AuthenticatorAttestationResponse) {
    return {
      ...base,
      response: {
        clientDataJSON: bufferToBase64url(response.clientDataJSON),
        attestationObject: bufferToBase64url(response.attestationObject),
        transports: response.getTransports?.() ?? [],
      },
    }
  }
  const assertion = response as AuthenticatorAssertionResponse
  return {
    ...base,
    response: {
      clientDataJSON: bufferToBase64url(assertion.clientDataJSON),
      authenticatorData: bufferToBase64url(assertion.authenticatorData),
      signature: bufferToBase64url(assertion.signature),
      userHandle: assertion.userHandle ? bufferToBase64url(assertion.userHandle) : undefined,
    },
  }
}

export function isWebAuthnSupported(): boolean {
  return typeof window !== 'undefined' && !!window.PublicKeyCredential
}

// Whether this browser can show passkeys as an autofill suggestion on a form
// field (mediation: "conditional") — required before starting a discoverable
// login ceremony on page load, since older/other browsers support WebAuthn
// generally but not this specific conditional-UI mode.
export async function isConditionalMediationAvailable(): Promise<boolean> {
  if (!isWebAuthnSupported() || typeof PublicKeyCredential.isConditionalMediationAvailable !== 'function') {
    return false
  }
  try {
    return await PublicKeyCredential.isConditionalMediationAvailable()
  } catch {
    return false
  }
}

// Runs navigator.credentials.create() against server-issued options and
// returns the JSON payload ready to POST to the register/finish endpoint.
export async function createWebAuthnCredential(serverOptions: unknown): Promise<Record<string, unknown>> {
  const options = decodeCreationOptions(serverOptions as ServerCreationOptions)
  const credential = await navigator.credentials.create(options) as PublicKeyCredential | null
  if (!credential) throw new Error('La création de la clé de sécurité a été annulée.')
  return encodeCredential(credential)
}

export interface GetWebAuthnAssertionOptions {
  // "conditional" makes the browser present matching passkeys as a form-field
  // autofill suggestion instead of a modal — used for the usernameless login.
  mediation?: CredentialMediationRequirement
  // Lets the caller cancel a long-lived conditional request (e.g. the login
  // page unmounting, or the user submitting the classic form instead).
  signal?: AbortSignal
}

// Runs navigator.credentials.get() against server-issued options and returns
// the JSON payload ready to POST to the login/finish endpoint.
export async function getWebAuthnAssertion(
  serverOptions: unknown,
  opts: GetWebAuthnAssertionOptions = {},
): Promise<Record<string, unknown>> {
  const options = decodeRequestOptions(serverOptions as ServerRequestOptions)
  if (opts.mediation) options.mediation = opts.mediation
  if (opts.signal) options.signal = opts.signal
  const credential = await navigator.credentials.get(options) as PublicKeyCredential | null
  if (!credential) throw new Error('La vérification de la clé de sécurité a été annulée.')
  return encodeCredential(credential)
}
