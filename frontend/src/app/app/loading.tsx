import { TableSkeleton } from "@/components/ui/states";
import { Skeleton } from "@/components/ui/skeleton";

export default function AppLoading() {
  return (
    <div className="space-y-6 animate-pulse">
      <div className="space-y-2">
        <Skeleton className="h-7 w-56 rounded-lg" />
        <Skeleton className="h-4 w-96 rounded-md" />
      </div>
      <TableSkeleton rows={5} />
    </div>
  );
}

