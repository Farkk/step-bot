export type Gender = '' | 'male' | 'female'
export type ProfileInput = { fullName: string; phone: string; gender: Gender; age: string }
export type ProfileErrors = Partial<Record<keyof ProfileInput, string>>

export function validateProfile(input: ProfileInput): ProfileErrors {
  const errors: ProfileErrors = {}
  const name = input.fullName.trim().replace(/\s+/g, ' ')
  if (!name) errors.fullName = 'Укажите ФИО'
  else if (name.length > 150 || !/^\p{L}[\p{L}'’-]{1,}(?:\s+\p{L}[\p{L}'’-]{1,})+$/u.test(name)) errors.fullName = 'Укажите фамилию и имя'
  const phone = input.phone.replace(/[\s()\-]/g, '')
  if (!phone) errors.phone = 'Укажите телефон'
  else if (!/^\+7\d{10}$/.test(phone)) errors.phone = 'Проверьте номер телефона'
  if (input.gender !== 'male' && input.gender !== 'female') errors.gender = 'Выберите пол'
  if (!input.age) errors.age = 'Укажите возраст'
  else if (!/^\d+$/.test(input.age) || Number(input.age) < 1 || Number(input.age) > 120) errors.age = 'Укажите возраст от 1 до 120 лет'
  return errors
}
