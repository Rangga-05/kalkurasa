"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { getMe } from "@/lib/api";
import { getDashboardPathForRole } from "@/lib/format";
import { Skeleton } from "@/components/ui/skeleton";

export default function AppIndexRedirectPage() {
  const router = useRouter();
  const meQuery = useQuery({
    queryKey: ["auth-me"],
    queryFn: ({ signal }) => getMe(signal),
  });

  useEffect(() => {
    if (meQuery.data) {
      router.replace(getDashboardPathForRole(meQuery.data.role));
    }
  }, [meQuery.data, router]);

  return (
    <div className="space-y-4">
      <Skeleton className="h-8 w-56" />
      <Skeleton className="h-40 w-full rounded-xl" />
    </div>
  );
}

