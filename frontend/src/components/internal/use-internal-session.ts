"use client";

import { useQuery } from "@tanstack/react-query";
import { getMe } from "@/lib/api";
import type { User } from "@/lib/types";

export function useInternalSession(): {
  user: User | undefined;
  isLoading: boolean;
} {
  const meQuery = useQuery({
    queryKey: ["auth-me"],
    queryFn: ({ signal }) => getMe(signal),
    retry: false,
  });
  return { user: meQuery.data, isLoading: meQuery.isLoading };
}

