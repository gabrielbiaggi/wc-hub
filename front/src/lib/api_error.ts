import { isAxiosError } from "axios";

type ErrorPayload = { error?: { message?: unknown } };

export function apiErrorMessage(error: unknown, fallback: string): string {
  if (isAxiosError<ErrorPayload>(error)) {
    const message = error.response?.data?.error?.message;
    if (typeof message === "string" && message.trim() !== "") return message;
    if (typeof error.message === "string" && error.message.trim() !== "")
      return error.message;
  }
  if (error instanceof Error && error.message.trim() !== "")
    return error.message;
  return fallback;
}
