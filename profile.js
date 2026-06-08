// profile.js
import { supabase } from './supabase.js'

// Récupérer le profil de l'utilisateur connecté
export async function getProfile() {
  const { data: { user } } = await supabase.auth.getUser()
  if (!user) return null

  const { data, error } = await supabase
    .from('profils')
    .select('*')
    .eq('id', user.id)
    .single()

  if (error) throw error
  return data
}

// Mettre à jour le profil
export async function updateProfile(updates) {
  const { data: { user } } = await supabase.auth.getUser()
  if (!user) throw new Error('Not authenticated')

  const { error } = await supabase
    .from('profils')
    .update(updates)
    .eq('id', user.id)

  if (error) throw error
}

// Vérifier si l'utilisateur est admin
export async function isAdmin() {
  const profile = await getProfile()
  return profile?.role === 'admin'
}

// Upload avatar
export async function uploadAvatar(file) {
  const { data: { user } } = await supabase.auth.getUser()
  if (!user) throw new Error('Not authenticated')

  const filePath = `avatars/${user.id}.png`

  const { error: uploadError } = await supabase.storage
    .from('avatars')
    .upload(filePath, file, { upsert: true })

  if (uploadError) throw uploadError

  const { data: urlData } = supabase.storage
    .from('avatars')
    .getPublicUrl(filePath)

  await updateProfile({ avatar_url: urlData.publicUrl })

  return urlData.publicUrl
}
