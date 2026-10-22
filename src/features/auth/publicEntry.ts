export function entryDestination(user: { id: number } | null, isLoading: boolean): "/login" | "/mode" | null {
  if (isLoading) return null;
  return user ? "/mode" : "/login";
}

export function registrationPayload(values: { email: string; password: string; name: string }, archetype: { id: string; image: string }) {
  return { email: values.email, password: values.password, name: values.name, image: archetype.image, characterType: archetype.id };
}
