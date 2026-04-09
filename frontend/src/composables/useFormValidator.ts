import { ref, computed } from 'vue'

export type ValidationRule = {
  validate: (value: unknown) => string | null
}

export function useFormValidator() {
  const fieldErrors = ref<Record<string, string>>({})

  const hasErrors = computed(() => Object.keys(fieldErrors.value).length > 0)

  const setFieldError = (fieldName: string, error: string | null) => {
    if (error) {
      fieldErrors.value[fieldName] = error
    } else {
      delete fieldErrors.value[fieldName]
    }
  }

  const clearErrors = () => {
    fieldErrors.value = {}
  }

  const validateField = (fieldName: string, value: unknown, rules: ValidationRule[]) => {
    for (const rule of rules) {
      const error = rule.validate(value)
      if (error) {
        setFieldError(fieldName, error)
        return false
      }
    }
    setFieldError(fieldName, null)
    return true
  }

  return {
    fieldErrors,
    hasErrors,
    setFieldError,
    clearErrors,
    validateField
  }
}

export const rules = {
  required: (fieldName: string): ValidationRule => ({
    validate: (value) => !value ? `${fieldName} est requis` : null
  }),
  minLength: (min: number): ValidationRule => ({
    validate: (value) => {
      if (typeof value !== 'string') return null
      return value.length < min ? `Minimum ${min} caractères` : null
    }
  }),
  email: (): ValidationRule => ({
    validate: (value) => {
      if (typeof value !== 'string') return null
      return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value) ? null : 'Email invalide'
    }
  }),
  url: (): ValidationRule => ({
    validate: (value) => {
      if (typeof value !== 'string') return null
      try {
        new URL(value)
        return null
      } catch {
        return 'URL invalide'
      }
    }
  })
}
