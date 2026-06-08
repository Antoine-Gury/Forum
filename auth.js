// auth.js
import { supabase } from './supabase.js'

// Inscription + création du profil
export async function signUp(email, password, username) {
  const { data: authData, error: authError } = await supabase.auth.signUp({
    email,
    password
  })

  if (authError) throw authError

  const user = authData.user

  // Création du profil lié à auth.users
  const { error: profileError } = await supabase.from('profils').insert({
    id: user.id,
    username: username || email.split('@')[0],
    role: 'user'
  })

  if (profileError) throw profileError

  return user
}

// Connexion
export async function signIn(email, password) {
  const { data, error } = await supabase.auth.signInWithPassword({
    email,
    password
  })
  if (error) throw error
  return data.user
}

// Déconnexion
export async function signOut() {
  await supabase.auth.signOut()
}
