import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import * as z from "zod";
import { Link, useNavigate } from "react-router-dom";
import { useAuth } from "@/context/AuthContext";
import { signInWithGithub } from "@/lib/firebase";
import { AuthLayout } from "@/components/auth/AuthLayout";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { useToast } from "@/hooks/useToast";
import { Mail, Lock } from "lucide-react";

const loginSchema = z.object({
    email: z.string().email("Please enter a valid email address"),
    password: z.string().min(1, "Password is required"),
});

type LoginForm = z.infer<typeof loginSchema>;

export function LoginPage() {
    const { login } = useAuth();
    const navigate = useNavigate();
    const { toast } = useToast();
    const [isLoading, setIsLoading] = useState(false);

    const {
        register,
        handleSubmit,
        formState: { errors },
    } = useForm<LoginForm>({
        resolver: zodResolver(loginSchema),
    });

    const onSubmit = async (data: LoginForm) => {
        setIsLoading(true);
        try {
            await login(data.email, data.password);
            toast({
                title: "Welcome back!",
                description: "You have successfully logged in.",
                variant: "success",
            });
            navigate("/dashboard");
        } catch (error) {
            toast({
                title: "Login failed",
                description: "Invalid email or password.",
                variant: "destructive",
            });
        } finally {
            setIsLoading(false);
        }
    };

    const handleGithubLogin = async () => {
        setIsLoading(true);
        try {
            await signInWithGithub();
            toast({
                title: "Welcome back!",
                description: "You have successfully logged in with GitHub.",
                variant: "success",
            });
            navigate("/dashboard");
        } catch (error) {
            console.error(error);
            toast({
                title: "Login failed",
                description: "Could not login with GitHub.",
                variant: "destructive",
            });
        } finally {
            setIsLoading(false);
        }
    };

    return (
        <AuthLayout
            title="Welcome back"
            description="Enter your email to sign in to your dashboard"
        >
            <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
                <Input
                    placeholder="name@example.com"
                    type="email"
                    autoCapitalize="none"
                    autoComplete="email"
                    autoCorrect="off"
                    label="Email"
                    icon={<Mail className="h-4 w-4" />}
                    error={errors.email?.message}
                    {...register("email")}
                />

                <div className="space-y-2">
                    <div className="flex items-center justify-between">
                        <label className="text-sm font-medium leading-none text-text-secondary">Password</label>
                        <Link to="/forgot-password" className="text-sm font-medium text-accent hover:underline">
                            Forgot password?
                        </Link>
                    </div>
                    <Input
                        placeholder="Password"
                        type="password"
                        autoComplete="current-password"
                        icon={<Lock className="h-4 w-4" />}
                        error={errors.password?.message}
                        {...register("password")}
                    />
                </div>

                <Button className="w-full" type="submit" isLoading={isLoading}>
                    Sign In
                </Button>
            </form>

            <div className="relative my-4">
                <div className="absolute inset-0 flex items-center">
                    <span className="w-full border-t border-dark-border" />
                </div>
                <div className="relative flex justify-center text-xs uppercase">
                    <span className="bg-dark-bg px-2 text-text-secondary">Or continue with</span>
                </div>
            </div>

            <Button variant="secondary" className="w-full gap-2" onClick={handleGithubLogin} isLoading={isLoading}>
                {/* Github Icon */}
                <svg role="img" viewBox="0 0 24 24" className="h-4 w-4 fill-current"><path d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12" /></svg>
                GitHub
            </Button>

            <div className="text-center text-sm text-text-secondary mt-6">
                Don&apos;t have an account?{" "}
                <Link to="/register" className="font-medium text-accent hover:underline">
                    Sign up
                </Link>
            </div>
        </AuthLayout>
    );
}
