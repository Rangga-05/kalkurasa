"use client";

import { useCallback, useEffect, useState } from "react";
import {
  CheckCircle2,
  Loader2,
  Plus,
  RefreshCw,
  UserCheck,
  UserMinus,
  Users,
} from "lucide-react";
import {
  createUser,
  deleteUser,
  getErrorMessage,
  getUsers,
  updateUser,
} from "@/lib/api";
import { formatDateTimeWIB, ROLE_LABELS } from "@/lib/format";
import type { User, UserRole } from "@/lib/types";
import { useInternalSession } from "@/components/internal/use-internal-session";
import { ErrorState, TableSkeleton } from "@/components/ui/states";
import { useToast } from "@/components/providers";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

export default function UsersManagementPage() {
  const { user: currentUser } = useInternalSession();
  const toast = useToast();

  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);

  // State form buat akun baru
  const [newName, setNewName] = useState("");
  const [newEmail, setNewEmail] = useState("");
  const [newRole, setNewRole] = useState<UserRole>("staff");
  const [newPassword, setNewPassword] = useState("");
  const [creating, setCreating] = useState(false);

  const loadUsers = useCallback(async () => {
    setLoading(true);
    setErrorMsg(null);
    try {
      const res = await getUsers({ page: 1, limit: 100 });
      setUsers(res.data);
    } catch (err) {
      setErrorMsg(getErrorMessage(err, "Gagal memuat daftar pengguna."));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadUsers();
  }, [loadUsers]);

  async function handleCreateUser(e: React.FormEvent) {
    e.preventDefault();
    if (!newName.trim() || !newEmail.trim() || newPassword.length < 8) {
      toast.error(
        "Validasi Gagal",
        "Nama, email, dan kata sandi (minimal 8 karakter) wajib diisi."
      );
      return;
    }
    setCreating(true);
    try {
      const created = await createUser({
        name: newName.trim(),
        email: newEmail.trim().toLowerCase(),
        role: newRole,
        password: newPassword,
        is_active: true,
      });
      toast.success(
        "Pengguna Dibuat",
        `Akun ${created.name} (${ROLE_LABELS[created.role]}) berhasil ditambahkan.`
      );
      setNewName("");
      setNewEmail("");
      setNewPassword("");
      setNewRole("staff");
      await loadUsers();
    } catch (err) {
      toast.error("Gagal Membuat Pengguna", getErrorMessage(err));
    } finally {
      setCreating(false);
    }
  }

  async function handleRoleChange(u: User, role: UserRole) {
    setBusyId(u.id);
    try {
      await updateUser(u.id, {
        name: u.name,
        email: u.email,
        role,
        is_active: u.is_active,
      });
      toast.success(
        "Peran Diperbarui",
        `Peran ${u.name} diubah menjadi ${ROLE_LABELS[role]}.`
      );
      await loadUsers();
    } catch (err) {
      toast.error("Gagal Mengubah Peran", getErrorMessage(err));
    } finally {
      setBusyId(null);
    }
  }

  async function handleToggleActive(u: User) {
    setBusyId(u.id);
    try {
      if (u.is_active) {
        await deleteUser(u.id);
        toast.info("Akun Dinonaktifkan", `Akun ${u.name} telah dinonaktifkan.`);
      } else {
        await updateUser(u.id, {
          name: u.name,
          email: u.email,
          role: u.role,
          is_active: true,
        });
        toast.success("Akun Diaktifkan", `Akun ${u.name} kembali aktif.`);
      }
      await loadUsers();
    } catch (err) {
      toast.error("Gagal Mengubah Status Akun", getErrorMessage(err));
    } finally {
      setBusyId(null);
    }
  }

  if (loading && users.length === 0) {
    return (
      <div className="space-y-6">
        <h1 className="text-2xl font-bold text-stone-900">
          Manajemen Pengguna &amp; Peran (RBAC)
        </h1>
        <TableSkeleton rows={6} />
      </div>
    );
  }

  if (errorMsg && users.length === 0) {
    return (
      <ErrorState
        title="Gagal Memuat Pengguna"
        message={errorMsg}
        onRetry={() => void loadUsers()}
      />
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <Badge variant="amber">Khusus Administrator</Badge>
          <h1 className="mt-1.5 text-2xl font-bold tracking-tight text-stone-900">
            Manajemen Pengguna &amp; Peran (RBAC)
          </h1>
          <p className="mt-0.5 text-sm text-stone-600">
            Tambah akun internal baru, sesuaikan peran otorisasi, atau
            nonaktifkan akun pengguna.
          </p>
        </div>

        <Button
          variant="secondary"
          size="sm"
          onClick={() => void loadUsers()}
          disabled={loading}
        >
          <RefreshCw
            className={`h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`}
          />
          Segarkan ({users.length})
        </Button>
      </div>

      {/* Form Tambah Pengguna */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-base">
            <Plus className="h-4 w-4 text-amber-700" />
            Tambah Akun Pengguna Baru
          </CardTitle>
          <CardDescription>
            Setiap penambahan dan perubahan peran akun dicatat secara otomatis
            ke dalam tabel audit_logs.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form
            onSubmit={handleCreateUser}
            className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-5"
          >
            <div>
              <Label htmlFor="new-name">Nama Lengkap</Label>
              <Input
                id="new-name"
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                placeholder="Nama staf/manajer"
                required
              />
            </div>
            <div>
              <Label htmlFor="new-email">Alamat Email</Label>
              <Input
                id="new-email"
                type="email"
                value={newEmail}
                onChange={(e) => setNewEmail(e.target.value)}
                placeholder="email@demo.test"
                required
              />
            </div>
            <div>
              <Label htmlFor="new-role">Peran (Role)</Label>
              <select
                id="new-role"
                value={newRole}
                onChange={(e) => setNewRole(e.target.value as UserRole)}
                className="mt-1 w-full rounded-xl border border-stone-300 bg-white px-3 py-2 text-xs font-medium text-stone-900"
              >
                <option value="owner">Pemilik Usaha (Owner)</option>
                <option value="manager">Manajer Operasional</option>
                <option value="staff">Karyawan Dapur &amp; Kurir</option>
                <option value="admin">Administrator Sistem</option>
              </select>
            </div>
            <div>
              <Label htmlFor="new-password">Kata Sandi (Min. 8 Karakter)</Label>
              <Input
                id="new-password"
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                placeholder="Minimal 8 karakter"
                required
              />
            </div>
            <div className="flex items-end">
              <Button type="submit" disabled={creating} className="w-full">
                {creating ? (
                  <>
                    <Loader2 className="h-4 w-4 animate-spin" />
                    Menyimpan...
                  </>
                ) : (
                  <>
                    <Plus className="h-4 w-4" />
                    Buat Akun
                  </>
                )}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      {/* Tabel Daftar Pengguna */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-base">
            <Users className="h-4 w-4 text-amber-700" />
            Daftar Akun Terdaftar ({users.length})
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs">
              <thead>
                <tr className="border-b border-stone-200 text-stone-500">
                  <th className="py-2.5 pr-3 font-semibold">Nama</th>
                  <th className="px-3 py-2.5 font-semibold">Email</th>
                  <th className="px-3 py-2.5 font-semibold">Peran</th>
                  <th className="px-3 py-2.5 font-semibold">Status</th>
                  <th className="px-3 py-2.5 font-semibold">Terdaftar</th>
                  <th className="py-2.5 pl-3 text-right font-semibold">Aksi</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-stone-100">
                {users.map((u) => {
                  const isSelf = currentUser?.id === u.id;
                  const isBusy = busyId === u.id;
                  return (
                    <tr key={u.id} className="hover:bg-stone-50/80">
                      <td className="py-2.5 pr-3 font-semibold text-stone-900">
                        {u.name}
                        {isSelf ? (
                          <span className="ml-1.5 text-[10px] font-bold text-amber-700">
                            (Anda)
                          </span>
                        ) : null}
                      </td>
                      <td className="px-3 py-2.5 font-mono text-stone-600">
                        {u.email}
                      </td>
                      <td className="px-3 py-2.5">
                        {u.role === "customer" || isSelf ? (
                          <Badge variant="amber">
                            {ROLE_LABELS[u.role] ?? u.role}
                          </Badge>
                        ) : (
                          <select
                            value={u.role}
                            disabled={isBusy}
                            onChange={(e) =>
                              void handleRoleChange(
                                u,
                                e.target.value as UserRole
                              )
                            }
                            className="rounded-lg border border-stone-200 bg-white px-2 py-1 text-xs font-semibold text-stone-800"
                          >
                            <option value="owner">Owner</option>
                            <option value="manager">Manager</option>
                            <option value="staff">Staff</option>
                            <option value="admin">Admin</option>
                          </select>
                        )}
                      </td>
                      <td className="px-3 py-2.5">
                        <Badge variant={u.is_active ? "emerald" : "slate"}>
                          {u.is_active ? "Aktif" : "Nonaktif"}
                        </Badge>
                      </td>
                      <td className="px-3 py-2.5 text-stone-500">
                        {formatDateTimeWIB(u.created_at)}
                      </td>
                      <td className="py-2.5 pl-3 text-right">
                        {!isSelf ? (
                          <Button
                            variant="outline"
                            size="sm"
                            disabled={isBusy}
                            onClick={() => void handleToggleActive(u)}
                          >
                            {u.is_active ? (
                              <>
                                <UserMinus className="h-3.5 w-3.5 text-rose-600" />
                                Nonaktifkan
                              </>
                            ) : (
                              <>
                                <UserCheck className="h-3.5 w-3.5 text-emerald-600" />
                                Aktifkan
                              </>
                            )}
                          </Button>
                        ) : (
                          <span className="inline-flex items-center gap-1 text-[11px] text-emerald-700">
                            <CheckCircle2 className="h-3.5 w-3.5" />
                            Sesi Aktif
                          </span>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
