function login(
  username: string,
  password: string,
  remember: boolean
): Promise<Response> {
  const form = new URLSearchParams();

  form.append("username", username);
  form.append("password", password);
  form.append("remember_me", String(remember));

  return fetch(`/apis/web/v1/login`, {
    method: "POST",
    body: form,
  });
}

function logout(): Promise<Response> {
  return fetch(`/apis/web/v1/logout`, {
    method: "POST",
  });
}

export {
  login,
  logout,
};